// SPDX-License-Identifier: AGPL-3.0-only

//go:build no_billing_aws

package config

func init() { exclude("no_billing_aws", "billing_sources", "aws") }

// checkBillingAWSTag accepts every value: no adapter of this build reads billing_aws_tag.
func checkBillingAWSTag(string) error { return nil }

// checkBillingAWSLinkedAccounts accepts every value: no adapter of this build reads
// billing_aws_linked_accounts.
func checkBillingAWSLinkedAccounts(string) error { return nil }

// checkBillingAWSRoleARN accepts every value: no adapter of this build reads billing_aws_role_arn.
func checkBillingAWSRoleARN(string) error { return nil }
