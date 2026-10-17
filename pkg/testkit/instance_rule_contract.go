// SPDX-License-Identifier: AGPL-3.0-only

package testkit

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"reflect"
	"strings"
	"testing"
	"time"

	appinstance "github.com/best-friends/my/internal/app/instance"
	"github.com/best-friends/my/internal/domain/instance"
)

// instanceRuleCases are the cases of InstanceRepositoryContract about the instance rules
// (FR-705): a rule keeps its ID across revisions, retired rules leave the rules in effect but
// stay readable, moves renumber the rules, texts follow the locale fallback chain, invalid
// input writes nothing, and the instance_rule_heads projection equals a rebuild.
var instanceRuleCases = []struct {
	name string
	run  func(t *testing.T, e *instanceEnv)
}{
	{"RuleRevisionsKeepTheRuleID", instanceRuleRevisionsKeepID},
	{"RetiredRulesLeaveTheRulesInEffect", instanceRetiredRules},
	{"MovesRules", instanceMovesRules},
	{"LocalizesRulesThroughTheFallbackChain", instanceLocalizesRules},
	{"RejectsInvalidRules", instanceRejectsInvalidRules},
	{"BoundsTheRulesInEffect", instanceBoundsRules},
	{"RuleProjectionEqualsRebuild", instanceRuleProjectionEqualsRebuild},
}

const ruleModerator = "0199f0e0-0000-7000-8000-0000000000aa"

func (e *instanceEnv) addRule(t *testing.T, texts ...instance.RuleText) instance.Rule {
	t.Helper()
	e.clock.Advance(time.Second)
	r, err := e.service.AddRule(t.Context(), appinstance.AddRuleInput{Texts: texts, CreatedByUserID: ruleModerator})
	if err != nil {
		t.Fatalf("AddRule(%+v) = %v", texts, err)
	}
	return r
}

func (e *instanceEnv) rules(t *testing.T, includeRetired bool) []instance.Rule {
	t.Helper()
	rules, err := e.service.Rules(t.Context(), includeRetired)
	if err != nil {
		t.Fatalf("Rules(%v) = %v", includeRetired, err)
	}
	return rules
}

// ruleOrder renders rules as "position:en text" in order.
func ruleOrder(rules []instance.Rule) []string {
	out := make([]string, len(rules))
	for i, r := range rules {
		t, _ := instance.PickRuleText(r.Revision.Texts, []string{"en"})
		out[i] = fmt.Sprintf("%d:%s", r.Revision.Position, t.Text)
	}
	return out
}

func en(text string) instance.RuleText { return instance.RuleText{Locale: "en", Text: text} }

func instanceRuleRevisionsKeepID(t *testing.T, e *instanceEnv) {
	ctx := t.Context()
	added := e.addRule(t, instance.RuleText{Locale: "ja_jp", Text: "親切に"}, instance.RuleText{Locale: "en", Text: "Be kind", Hint: "No harassment."})
	if want := []instance.RuleText{{Locale: "en", Text: "Be kind", Hint: "No harassment."}, {Locale: "ja-JP", Text: "親切に"}}; !reflect.DeepEqual(added.Revision.Texts, want) {
		t.Errorf("added texts = %+v, want the canonical locales by locale %+v", added.Revision.Texts, want)
	}
	if added.Revision.Position != 1 || added.Revision.RevisedByUserID != ruleModerator || added.Retired() {
		t.Errorf("added rule = %+v, want position 1 by the moderator, in effect", added)
	}

	e.clock.Advance(time.Second)
	revised, err := e.service.ReviseRule(ctx, appinstance.ReviseRuleInput{RuleID: added.ID, Texts: []instance.RuleText{en("Be very kind")}})
	if err != nil {
		t.Fatalf("ReviseRule = %v", err)
	}
	if revised.ID != added.ID || revised.Revision.ID == added.Revision.ID || revised.Revision.Position != 1 ||
		!reflect.DeepEqual(revised.Revision.Texts, []instance.RuleText{en("Be very kind")}) || revised.Revision.RevisedByUserID != "" {
		t.Errorf("revised = %+v; want the same rule ID with a new revision that replaces every text", revised)
	}

	e.clock.Advance(time.Second)
	set, err := e.service.SetRuleText(ctx, appinstance.SetRuleTextInput{RuleID: added.ID, Locale: "ja", Text: "とても親切に", Hint: "嫌がらせ禁止"})
	if err != nil {
		t.Fatalf("SetRuleText(ja) = %v", err)
	}
	e.clock.Advance(time.Second)
	set, err = e.service.SetRuleText(ctx, appinstance.SetRuleTextInput{RuleID: added.ID, Locale: "ja", Text: "常に親切に", KeepHint: true})
	if err != nil {
		t.Fatalf("SetRuleText(ja, keep hint) = %v", err)
	}
	want := []instance.RuleText{en("Be very kind"), {Locale: "ja", Text: "常に親切に", Hint: "嫌がらせ禁止"}}
	if !reflect.DeepEqual(set.Revision.Texts, want) {
		t.Errorf("SetRuleText texts = %+v, want %+v", set.Revision.Texts, want)
	}
	got, err := e.service.Rule(ctx, added.ID)
	if err != nil {
		t.Fatalf("Rule = %v", err)
	}
	if !reflect.DeepEqual(got, set) || !got.CreatedAt.Equal(added.CreatedAt) {
		t.Errorf("Rule = %+v\nwant  %+v", got, set)
	}
	// Removing a locale keeps the others; the last one cannot go.
	e.clock.Advance(time.Second)
	if r, err := e.service.SetRuleText(ctx, appinstance.SetRuleTextInput{RuleID: added.ID, Locale: "ja"}); err != nil ||
		!reflect.DeepEqual(r.Revision.Texts, []instance.RuleText{en("Be very kind")}) {
		t.Errorf("SetRuleText(remove ja) = %+v, %v; want only en", r.Revision.Texts, err)
	}
	if _, err := e.service.SetRuleText(ctx, appinstance.SetRuleTextInput{RuleID: added.ID, Locale: "en"}); !errors.Is(err, instance.ErrInvalidValue) {
		t.Errorf("SetRuleText(remove the last locale) = %v, want ErrInvalidValue", err)
	}
	if rules := e.rules(t, true); len(rules) != 1 || rules[0].ID != added.ID {
		t.Errorf("Rules = %+v, want the one rule", rules)
	}
}

func instanceRetiredRules(t *testing.T, e *instanceEnv) {
	ctx := t.Context()
	first := e.addRule(t, en("First"))
	second := e.addRule(t, en("Second"))
	e.clock.Advance(time.Second)
	retired, err := e.service.RetireRule(ctx, first.ID, ruleModerator)
	if err != nil {
		t.Fatalf("RetireRule = %v", err)
	}
	if !retired.Retired() || retired.Retirement.RetiredByUserID != ruleModerator || !retired.Retirement.RetiredAt.Equal(e.clock.Now()) {
		t.Errorf("retired = %+v, want retired by the moderator now", retired.Retirement)
	}
	// Retiring again changes nothing.
	e.clock.Advance(time.Second)
	again, err := e.service.RetireRule(ctx, first.ID, "")
	if err != nil || !reflect.DeepEqual(again, retired) {
		t.Errorf("RetireRule again = %+v, %v; want the first retirement %+v", again, err, retired)
	}

	if got := ruleOrder(e.rules(t, false)); !reflect.DeepEqual(got, []string{"2:Second"}) {
		t.Errorf("rules in effect = %q, want only the second", got)
	}
	if got, err := e.service.LocalizedRules(ctx, "en"); err != nil || len(got) != 1 || got[0].ID != second.ID {
		t.Errorf("LocalizedRules = %+v, %v; want only the second", got, err)
	}
	all := e.rules(t, true)
	if len(all) != 2 || all[0].ID != second.ID || all[1].ID != first.ID || !reflect.DeepEqual(all[1], retired) {
		t.Errorf("Rules(include retired) = %+v; want the second, then the retired first", all)
	}
	// Reports still find a retired rule by its ID.
	if got, err := e.service.Rule(ctx, first.ID); err != nil || !reflect.DeepEqual(got, retired) {
		t.Errorf("Rule(retired) = %+v, %v; want %+v", got, err, retired)
	}
	if _, err := e.service.ReviseRule(ctx, appinstance.ReviseRuleInput{RuleID: first.ID, Texts: []instance.RuleText{en("x")}}); !errors.Is(err, instance.ErrRuleRetired) {
		t.Errorf("ReviseRule(retired) = %v, want ErrRuleRetired", err)
	}
	if _, err := e.service.MoveRule(ctx, appinstance.MoveRuleInput{RuleID: first.ID, Position: 1}); !errors.Is(err, instance.ErrRuleRetired) {
		t.Errorf("MoveRule(retired) = %v, want ErrRuleRetired", err)
	}
	// A new rule goes after the rules in effect.
	if third := e.addRule(t, en("Third")); third.Revision.Position != 3 {
		t.Errorf("third position = %d, want 3", third.Revision.Position)
	}
}

func instanceMovesRules(t *testing.T, e *instanceEnv) {
	ctx := t.Context()
	a := e.addRule(t, en("A"))
	e.addRule(t, en("B"))
	c := e.addRule(t, en("C"), instance.RuleText{Locale: "ja", Text: "シー"})
	move := func(r instance.Rule, position int, want ...string) {
		t.Helper()
		e.clock.Advance(time.Second)
		moved, err := e.service.MoveRule(ctx, appinstance.MoveRuleInput{RuleID: r.ID, Position: position, RevisedByUserID: ruleModerator})
		if err != nil {
			t.Fatalf("MoveRule(%d) = %v", position, err)
		}
		if moved.ID != r.ID {
			t.Errorf("moved rule = %s, want %s", moved.ID, r.ID)
		}
		if got := ruleOrder(e.rules(t, false)); !reflect.DeepEqual(got, want) {
			t.Errorf("after MoveRule(%d): rules = %q, want %q", position, got, want)
		}
	}
	move(c, 1, "1:C", "2:A", "3:B")
	move(c, 99, "1:A", "2:B", "3:C")
	move(a, 2, "1:B", "2:A", "3:C")
	move(a, 2, "1:B", "2:A", "3:C")
	// Moving keeps the texts.
	got, err := e.service.Rule(ctx, c.ID)
	if err != nil || !reflect.DeepEqual(got.Revision.Texts, c.Revision.Texts) {
		t.Errorf("moved rule texts = %+v, %v; want %+v", got.Revision.Texts, err, c.Revision.Texts)
	}
	// Retiring leaves a gap that the next move closes.
	if _, err := e.service.RetireRule(ctx, a.ID, ""); err != nil {
		t.Fatalf("RetireRule = %v", err)
	}
	move(c, 1, "1:C", "2:B")
}

func instanceLocalizesRules(t *testing.T, e *instanceEnv) {
	ctx := t.Context()
	e.addRule(t, en("Be kind"), instance.RuleText{Locale: "ja", Text: "親切に", Hint: "補足"}, instance.RuleText{Locale: "zh-Hant", Text: "友善"})
	e.addRule(t, instance.RuleText{Locale: "fr", Text: "Soyez gentil"}, instance.RuleText{Locale: "de", Text: "Sei nett"})
	check := func(locale string, want ...string) {
		t.Helper()
		rules, err := e.service.LocalizedRules(ctx, locale)
		if err != nil {
			t.Fatalf("LocalizedRules(%q) = %v", locale, err)
		}
		var got []string
		for _, r := range rules {
			got = append(got, r.Text.Locale+":"+r.Text.Text+"|"+r.Text.Hint)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("LocalizedRules(%q) = %q, want %q", locale, got, want)
		}
	}
	// A rule in no locale of the chain shows in its first locale.
	check("ja-JP", "ja:親切に|補足", "de:Sei nett|")
	check("zh-TW", "zh-Hant:友善|", "de:Sei nett|")
	check("fr", "en:Be kind|", "fr:Soyez gentil|")
	check("", "en:Be kind|", "de:Sei nett|")
	e.set(t, instance.SettingDefaultLocale, "ja", "")
	check("fr-CA", "ja:親切に|補足", "fr:Soyez gentil|")
	check("en-GB", "en:Be kind|", "de:Sei nett|")
}

func instanceRejectsInvalidRules(t *testing.T, e *instanceEnv) {
	ctx := t.Context()
	for _, tc := range []struct {
		texts []instance.RuleText
		index int
		field string
		max   int
	}{
		{nil, -1, "", 0},
		{[]instance.RuleText{{Locale: "", Text: "x"}}, 0, "locale", 0},
		{[]instance.RuleText{en("x"), {Locale: "e n", Text: "x"}}, 1, "locale", 0},
		{[]instance.RuleText{{Locale: "ja_jp", Text: "x"}, {Locale: "ja-JP", Text: "y"}}, 1, "locale", 0},
		{[]instance.RuleText{en("two\nlines")}, 0, "text", 0},
		{[]instance.RuleText{en("   ")}, 0, "text", 0},
		{[]instance.RuleText{en("nul\x00")}, 0, "text", 0},
		{[]instance.RuleText{en("\xff")}, 0, "text", 0},
		{[]instance.RuleText{en(strings.Repeat("a", instance.MaxRuleTextLength+1))}, 0, "text", instance.MaxRuleTextLength},
		{[]instance.RuleText{{Locale: "en", Text: "x", Hint: strings.Repeat("a", instance.MaxRuleHintLength+1)}}, 0, "hint", instance.MaxRuleHintLength},
		{[]instance.RuleText{{Locale: "en", Text: "x", Hint: "nul\x00"}}, 0, "hint", 0},
	} {
		_, err := e.service.AddRule(ctx, appinstance.AddRuleInput{Texts: tc.texts})
		var te *instance.RuleTextError
		if !errors.Is(err, instance.ErrInvalidValue) || !errors.As(err, &te) || te.Index != tc.index || te.Field != tc.field || te.Max != tc.max {
			t.Errorf("AddRule(%q) = %v, want a RuleTextError{%d, %q, %d}", tc.texts, err, tc.index, tc.field, tc.max)
		}
	}
	many := make([]instance.RuleText, instance.MaxRuleLocales+1)
	for i := range many {
		many[i] = instance.RuleText{Locale: fmt.Sprintf("%c%c", 'a'+i/26, 'a'+i%26), Text: "x"}
	}
	if _, err := e.service.AddRule(ctx, appinstance.AddRuleInput{Texts: many}); !errors.Is(err, instance.ErrInvalidValue) {
		t.Errorf("AddRule(%d locales) = %v, want ErrInvalidValue", len(many), err)
	}
	if rules := e.rules(t, true); len(rules) != 0 {
		t.Errorf("Rules = %+v, want nothing written", rules)
	}
	// The longest text is accepted, counted in characters.
	r := e.addRule(t, instance.RuleText{Locale: "ja", Text: strings.Repeat("あ", instance.MaxRuleTextLength), Hint: "two\nlines"})

	const unknown = "0199f0e0-0000-7000-8000-00000000dead"
	for name, err := range map[string]error{
		"Rule": func() error { _, err := e.service.Rule(ctx, unknown); return err }(),
		"ReviseRule": func() error {
			_, err := e.service.ReviseRule(ctx, appinstance.ReviseRuleInput{RuleID: unknown, Texts: []instance.RuleText{en("x")}})
			return err
		}(),
		"SetRuleText": func() error {
			_, err := e.service.SetRuleText(ctx, appinstance.SetRuleTextInput{RuleID: unknown, Locale: "en", Text: "x"})
			return err
		}(),
		"MoveRule": func() error {
			_, err := e.service.MoveRule(ctx, appinstance.MoveRuleInput{RuleID: unknown, Position: 1})
			return err
		}(),
		"RetireRule": func() error { _, err := e.service.RetireRule(ctx, unknown, ""); return err }(),
	} {
		if !errors.Is(err, appinstance.ErrNotFound) {
			t.Errorf("%s(unknown) = %v, want ErrNotFound", name, err)
		}
	}
	if _, err := e.service.MoveRule(ctx, appinstance.MoveRuleInput{RuleID: r.ID, Position: 0}); !errors.Is(err, instance.ErrInvalidValue) {
		t.Errorf("MoveRule(0) = %v, want ErrInvalidValue", err)
	}
	if _, err := e.service.ReviseRule(ctx, appinstance.ReviseRuleInput{RuleID: r.ID}); !errors.Is(err, instance.ErrInvalidValue) {
		t.Errorf("ReviseRule(no texts) = %v, want ErrInvalidValue", err)
	}
	if got, err := e.service.Rule(ctx, r.ID); err != nil || got.Revision.ID != r.Revision.ID {
		t.Errorf("Rule = %+v, %v; want the rule unchanged", got, err)
	}
}

func instanceBoundsRules(t *testing.T, e *instanceEnv) {
	ctx := t.Context()
	var first instance.Rule
	for i := range instance.MaxRules {
		r := e.addRule(t, en(fmt.Sprintf("rule %d", i)))
		if i == 0 {
			first = r
		}
	}
	if _, err := e.service.AddRule(ctx, appinstance.AddRuleInput{Texts: []instance.RuleText{en("one more")}}); !errors.Is(err, instance.ErrTooManyRules) {
		t.Errorf("AddRule beyond %d = %v, want ErrTooManyRules", instance.MaxRules, err)
	}
	// Retired rules do not count.
	if _, err := e.service.RetireRule(ctx, first.ID, ""); err != nil {
		t.Fatalf("RetireRule = %v", err)
	}
	e.addRule(t, en("one more"))
}

func instanceRuleProjectionEqualsRebuild(t *testing.T, e *instanceEnv) {
	ctx := t.Context()
	rng := rand.New(rand.NewPCG(331, 20261017)) //nolint:gosec // deterministic test input, not a secret
	var ids []string
	for round := range 3 {
		for step := range 30 {
			e.clock.Advance(time.Second)
			text := en(fmt.Sprintf("rule %d.%d", round, step))
			var err error
			switch op := rng.IntN(5); {
			case op == 0 || len(ids) == 0:
				var r instance.Rule
				r, err = e.service.AddRule(ctx, appinstance.AddRuleInput{Texts: []instance.RuleText{text}})
				ids = append(ids, r.ID)
			case op == 1:
				_, err = e.service.ReviseRule(ctx, appinstance.ReviseRuleInput{RuleID: ids[rng.IntN(len(ids))], Texts: []instance.RuleText{text}})
			case op == 2:
				_, err = e.service.MoveRule(ctx, appinstance.MoveRuleInput{RuleID: ids[rng.IntN(len(ids))], Position: 1 + rng.IntN(len(ids)+1)})
			case op == 3 && rng.IntN(3) == 0:
				_, err = e.service.RetireRule(ctx, ids[rng.IntN(len(ids))], "")
			default:
				_, err = e.service.SetRuleText(ctx, appinstance.SetRuleTextInput{RuleID: ids[rng.IntN(len(ids))], Locale: "ja", Text: "ルール"})
			}
			if err != nil && !errors.Is(err, instance.ErrRuleRetired) {
				t.Fatalf("round %d step %d: %v", round, step, err)
			}
		}
		incremental := e.rules(t, false)
		if len(incremental) == 0 {
			t.Fatalf("round %d: no rules in effect", round)
		}
		if err := appinstance.RebuildProjections(ctx, e.h.UnitOfWork, e.h.Instance); err != nil {
			t.Fatalf("RebuildProjections = %v", err)
		}
		if rebuilt := e.rules(t, false); !reflect.DeepEqual(incremental, rebuilt) {
			t.Fatalf("round %d: incremental rules differ from a rebuild:\nincremental %+v\nrebuilt     %+v", round, incremental, rebuilt)
		}
	}
}
