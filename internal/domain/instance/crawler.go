// SPDX-License-Identifier: AGPL-3.0-only

package instance

// CrawlerPolicy is what a page states to search engines and AI crawlers (ADR-0057): the
// instance's settings, narrowed by those of the page's owner. Where statements conflict, the
// most restrictive one wins: an owner can deny what the instance allows, never the reverse.
type CrawlerPolicy struct {
	Search SearchIndexing
	AI     AIUsage
}

// DenyAll is the policy that denies everything, the instance's when it has no settings.
var DenyAll = CrawlerPolicy{Search: SearchDeny, AI: AIDeny}

// ForOwner narrows p by the settings of the account that owns the page: an account that is not
// indexable denies search, and one that opts out of AI denies its training and use.
func (p CrawlerPolicy) ForOwner(indexable, aiOptOut bool) CrawlerPolicy {
	if !indexable {
		p = p.NoIndex()
	}
	if aiOptOut {
		p.AI = AIDeny
	}
	return p
}

// NoIndex returns p with search denied, for the pages that search engines never index whatever
// the settings (unlisted posts, remote accounts, the app's screens, ADR-0057).
func (p CrawlerPolicy) NoIndex() CrawlerPolicy {
	p.Search = SearchDeny
	return p
}

// IndexAllowed reports whether search engines may index the page.
func (p CrawlerPolicy) IndexAllowed() bool { return p.Search == SearchAllow }

// DeniesTraining reports whether the page may not train AI models.
func (p CrawlerPolicy) DeniesTraining() bool { return p.AI != AIAllow }

// DeniesAIUse reports whether AI may not use the page at all (AI search, assistants, agents),
// training included.
func (p CrawlerPolicy) DeniesAIUse() bool { return p.AI != AIAllow && p.AI != AIDenyTraining }
