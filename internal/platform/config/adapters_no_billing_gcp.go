// SPDX-License-Identifier: AGPL-3.0-only

//go:build no_billing_gcp

package config

func init() { exclude("no_billing_gcp", "billing_sources", "gcp") }

// checkBillingGCPTable accepts every value: no adapter of this build reads billing_gcp_table.
func checkBillingGCPTable(string) error { return nil }

// checkBillingGCPProject accepts every value: no adapter of this build reads
// billing_gcp_query_project.
func checkBillingGCPProject(string) error { return nil }

// checkBillingGCPProjects accepts every value: no adapter of this build reads
// billing_gcp_projects.
func checkBillingGCPProjects(string) error { return nil }

// checkBillingGCPLabel accepts every value: no adapter of this build reads billing_gcp_label.
func checkBillingGCPLabel(string) error { return nil }
