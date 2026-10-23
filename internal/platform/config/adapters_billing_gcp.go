// SPDX-License-Identifier: AGPL-3.0-only

//go:build !no_billing_gcp

package config

import billinggcp "github.com/mycroblog/my/internal/adapter/billing/gcp"

func checkBillingGCPTable(v string) error { return billinggcp.CheckTable(v) }

func checkBillingGCPProject(v string) error { return billinggcp.CheckProject(v) }

func checkBillingGCPProjects(v string) error { return billinggcp.CheckProjects(v) }

func checkBillingGCPLabel(v string) error { return billinggcp.CheckLabel(v) }

func checkBillingGCPSubscription(v string) error { return billinggcp.CheckSubscription(v) }
