// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

// Package beats provides the beat-subcommand registration hook for the
// elastic-otel-collector binary. In the endpoint-security prototype build,
// AddCommands is a no-op and this package imports no beat CLI frameworks,
// keeping them out of the binary link graph entirely.
package beats
