// SPDX-License-Identifier: AGPL-3.0-only

package activitypub

import (
	"log/slog"
	"net/http"
)

// nodeInfoSchema names NodeInfo 2.1, as the link relation and the profile of the document.
const nodeInfoSchema = "http://nodeinfo.diaspora.software/ns/schema/2.1"

// softwareName is the server's NodeInfo software name (^[a-z0-9-]+$).
const softwareName = "my"

// nodeInfoIndex serves GET /.well-known/nodeinfo: the link to the NodeInfo 2.1 document.
func (h *Handler) nodeInfoIndex(w http.ResponseWriter, r *http.Request) {
	h.writeJSON(w, r, typeJSON, struct {
		Links []jrdLink `json:"links"`
	}{Links: []jrdLink{{Rel: nodeInfoSchema, Href: "https://" + h.o.Host + PathNodeInfo}}})
}

type nodeInfo struct {
	Version           string           `json:"version"`
	Software          nodeInfoSoftware `json:"software"`
	Protocols         []string         `json:"protocols"`
	Services          nodeInfoServices `json:"services"`
	OpenRegistrations bool             `json:"openRegistrations"`
	Usage             nodeInfoUsage    `json:"usage"`
	Metadata          map[string]any   `json:"metadata"`
}

type nodeInfoSoftware struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	// Repository is the source code that AGPL-3.0 §13 offers to users (ADR-0011).
	Repository string `json:"repository,omitempty"`
}

type nodeInfoServices struct {
	Inbound  []string `json:"inbound"`
	Outbound []string `json:"outbound"`
}

type nodeInfoUsage struct {
	Users      nodeInfoUsers `json:"users"`
	LocalPosts int64         `json:"localPosts"`
}

type nodeInfoUsers struct {
	Total int `json:"total"`
}

// nodeInfo serves GET /nodeinfo/2.1 (FR-402). openRegistrations follows the registration mode,
// as Mastodon reports it: anyone may sign up, with or without a moderator's approval. localPosts
// counts the local posts as Mastodon does (content.Service.LocalPostCount); active-user counts
// are omitted because logins are not counted.
func (h *Handler) nodeInfo(w http.ResponseWriter, r *http.Request) {
	users, err := h.o.Accounts.CountLocalUsers(r.Context())
	if err != nil {
		h.o.Logger.ErrorContext(r.Context(), "nodeinfo: count users failed", slog.Any("error", err))
		h.o.Problem(w, r, http.StatusInternalServerError)
		return
	}
	open := false
	if h.o.OpenRegistrations != nil {
		if open, err = h.o.OpenRegistrations(r.Context()); err != nil {
			h.o.Logger.ErrorContext(r.Context(), "nodeinfo: read the registration mode failed", slog.Any("error", err))
			h.o.Problem(w, r, http.StatusInternalServerError)
			return
		}
	}
	var posts int64
	if h.o.LocalPosts != nil {
		if posts, err = h.o.LocalPosts(r.Context()); err != nil {
			h.o.Logger.ErrorContext(r.Context(), "nodeinfo: count local posts failed", slog.Any("error", err))
			h.o.Problem(w, r, http.StatusInternalServerError)
			return
		}
	}
	// nodeName is the name that Mastodon and other servers read from NodeInfo's free-form
	// metadata to show the instance by.
	name := h.o.Host
	if h.o.InstanceName != nil {
		if n := h.o.InstanceName(r.Context()); n != "" {
			name = n
		}
	}
	w.Header().Set("Access-Control-Allow-Origin", "*")
	h.writeJSON(w, r, typeNodeInfo, nodeInfo{
		Version:           "2.1",
		Software:          nodeInfoSoftware{Name: softwareName, Version: h.o.SoftwareVersion, Repository: h.o.SourceURL},
		Protocols:         []string{"activitypub"},
		Services:          nodeInfoServices{Inbound: []string{}, Outbound: []string{}},
		OpenRegistrations: open,
		Usage:             nodeInfoUsage{Users: nodeInfoUsers{Total: users}, LocalPosts: posts},
		Metadata:          map[string]any{"nodeName": name},
	})
}
