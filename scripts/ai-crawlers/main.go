// SPDX-License-Identifier: AGPL-3.0-only

// Command ai-crawlers generates the table of AI crawlers that /robots.txt names (ADR-0057), from
// the robots.json of ai-robots-txt/ai.robots.txt (MIT) at a commit. `scripts/task.sh go:ai-crawlers` downloads
// the file and runs it:
//
//	go run ./scripts/ai-crawlers -in robots.json -commit <sha> -out internal/interface/web/ai_crawlers.gen.go
//
// Every crawler is classified by its purpose: training (collecting data to train AI models) or
// use (AI search, assistants, and agents acting for a user). The classification reads the
// list's free-text "function", so it is a heuristic: a crawler whose function mentions
// training, models, or datasets is a training crawler, one that mentions search, assistants,
// agents, or users is a use crawler, and anything else is a training crawler, which every
// reservation of AI denies. overrides corrects the entries that the heuristic misreads. Link
// preview fetchers and general search engines are left out (excluded), so that a reservation of
// AI never stops link previews or search, which the instance states with noindex instead.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"go/format"
	"log"
	"os"
	"slices"
	"strings"
)

// upstreamLicense is the license of the list, which the generated file carries.
const upstreamLicense = `// The tables below are made from ai.robots.txt, under its license:
//
// MIT License
//
// Copyright (c) 2024 ai.robots.txt
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.
`

// Purposes of a crawler.
const (
	training = "training"
	use      = "use"
)

// excluded are the link preview fetchers and general search engines, which the table never
// names (ADR-0057).
var excluded = map[string]bool{
	"facebookexternalhit": true, "Facebot": true, "Twitterbot": true, "Discordbot": true, "Slackbot": true,
	"Slackbot-LinkExpanding": true, "TelegramBot": true, "WhatsApp": true, "LinkedInBot": true,
	"Googlebot": true, "Bingbot": true, "bingbot": true, "Applebot": true, "DuckDuckBot": true, "YandexBot": true,
	"Baiduspider": true,
}

// overrides are the purposes of the crawlers whose function the heuristic misreads.
var overrides = map[string]string{
	// The function names Alexa's answers, but Amazon trains its models with it.
	"Amazonbot": training,
	// The function says that Mistral does not train with it.
	"MistralAI-Index": use,
}

var (
	trainingWords = []string{"train", "llm", "language model", "dataset", "machine learning", "model"}
	useWords      = []string{"search", "assistant", "agent", "user", "answer"}
)

// purpose classifies a crawler by its name and function.
func purpose(name, function string) string {
	if p, ok := overrides[name]; ok {
		return p
	}
	// "research" is not search.
	f := strings.ReplaceAll(strings.ToLower(function), "research", "")
	for _, w := range trainingWords {
		if strings.Contains(f, w) {
			return training
		}
	}
	for _, w := range useWords {
		if strings.Contains(f, w) {
			return use
		}
	}
	return training
}

func main() {
	in := flag.String("in", "", "the robots.json to read")
	out := flag.String("out", "", "the Go file to write")
	commit := flag.String("commit", "", "the commit of ai-robots-txt/ai.robots.txt that the input is from, recorded in the output")
	flag.Parse()
	if *in == "" || *out == "" || *commit == "" {
		log.Fatal("ai-crawlers: -in, -out, and -commit are required")
	}
	src, err := os.ReadFile(*in)
	if err != nil {
		log.Fatal(err)
	}
	var list map[string]struct {
		Function string `json:"function"`
	}
	if err := json.Unmarshal(src, &list); err != nil {
		log.Fatalf("ai-crawlers: %s: %v", *in, err)
	}
	byPurpose := map[string][]string{}
	for name, c := range list {
		name = strings.TrimSpace(name)
		// A name is a line of robots.txt: it may not break the line or start a comment.
		if name == "" || excluded[name] || strings.ContainsAny(name, "\r\n#") {
			continue
		}
		p := purpose(name, c.Function)
		byPurpose[p] = append(byPurpose[p], name)
	}
	var b bytes.Buffer
	fmt.Fprintf(&b, "// Code generated by scripts/ai-crawlers from ai-robots-txt/ai.robots.txt %s; DO NOT EDIT.\n\n", *commit)
	b.WriteString(upstreamLicense)
	b.WriteString("\npackage web\n\n")
	fmt.Fprintf(&b, "// aiCrawlersSource is the list of ai-robots-txt/ai.robots.txt (MIT) that the tables are made from.\nconst aiCrawlersSource = %q\n\n",
		"https://github.com/ai-robots-txt/ai.robots.txt/blob/"+*commit+"/robots.json")
	for _, p := range []struct{ name, purpose, doc string }{
		{"aiTrainingCrawlers", training, "the crawlers that collect data to train AI models"},
		{"aiUseCrawlers", use, "the crawlers of AI search, assistants, and agents acting for a user"},
	} {
		names := byPurpose[p.purpose]
		slices.SortFunc(names, func(a, b string) int {
			if c := strings.Compare(strings.ToLower(a), strings.ToLower(b)); c != 0 {
				return c
			}
			return strings.Compare(a, b)
		})
		fmt.Fprintf(&b, "// %s are %s.\nvar %s = []string{\n", p.name, p.doc, p.name)
		for _, n := range names {
			fmt.Fprintf(&b, "\t%q,\n", n)
		}
		b.WriteString("}\n\n")
	}
	formatted, err := format.Source(b.Bytes())
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(*out, formatted, 0o600); err != nil {
		log.Fatal(err)
	}
}
