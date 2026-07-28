// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

//go:build !requirefips

package beats

import "github.com/spf13/cobra"

// AddCommands is a no-op in this endpoint-security prototype build.
// Beat subcommands are removed to reduce binary size; beats run only as
// OTel receivers (fbreceiver, mbreceiver, abreceiver, osqreceiver).
func AddCommands(_ *cobra.Command) {}
