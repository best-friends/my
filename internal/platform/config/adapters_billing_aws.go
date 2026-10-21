// SPDX-License-Identifier: AGPL-3.0-only

//go:build !no_billing_aws

package config

import billingaws "github.com/best-friends/my/internal/adapter/billing/aws"

func checkBillingAWSTag(v string) error { return billingaws.CheckTag(v) }

func checkBillingAWSLinkedAccounts(v string) error { return billingaws.CheckLinkedAccounts(v) }

func checkBillingAWSRoleARN(v string) error { return billingaws.CheckRoleARN(v) }
