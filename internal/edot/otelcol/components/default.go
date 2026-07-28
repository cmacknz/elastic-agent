// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

// Package components hosts the default EDOT collector component registry. It
// is intentionally split out of the otelcol package so that callers (such as
// the manager unit-test binary) can wire up a smaller component set without
// dragging the full set of receivers, processors, exporters, and extensions
// into their build.
//
// This is an endpoint-security prototype build: only beats-based receivers
// (filebeat, metricbeat, auditbeat) and the components required for config
// translation are included. See internal/pkg/otel/translate for the
// translation layer that drives the included component set.
package components

import (
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/otelcol"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/service/telemetry/otelconftelemetry"

	"github.com/elastic/elastic-agent/internal/pkg/agent/application/paths"

	// Receivers — beats-based only for endpoint:
	fbreceiver "github.com/elastic/beats/v7/x-pack/filebeat/fbreceiver"
	mbreceiver "github.com/elastic/beats/v7/x-pack/metricbeat/mbreceiver"
	nopreceiver "go.opentelemetry.io/collector/receiver/nopreceiver"
	otlpreceiver "go.opentelemetry.io/collector/receiver/otlpreceiver"

	// Processors — subset needed for config translation:
	transformprocessor "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor"

	"github.com/elastic/beats/v7/x-pack/otel/processor/beatprocessor"

	// Exporters — elasticsearch, logstash, kafka outputs (from translate package):
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter"
	debugexporter "go.opentelemetry.io/collector/exporter/debugexporter"
	nopexporter "go.opentelemetry.io/collector/exporter/nopexporter"

	"github.com/elastic/beats/v7/x-pack/otel/exporter/logstashexporter"

	// Extensions — beats auth + storage (from translate package) + basic infrastructure:
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/bearertokenauthextension"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextension"
	pprofextension "github.com/open-telemetry/opentelemetry-collector-contrib/extension/pprofextension"
	filestorage "github.com/open-telemetry/opentelemetry-collector-contrib/extension/storage/filestorage"
	"go.opentelemetry.io/collector/extension/memorylimiterextension"

	elasticsearchstorage "github.com/elastic/beats/v7/x-pack/otel/extension/elasticsearchstorage"
	kafkapartitionerextension "github.com/elastic/beats/v7/x-pack/otel/extension/kafkapartitionerextension"
	elasticdiagnostics "github.com/elastic/elastic-agent/internal/pkg/otel/extension/elasticdiagnostics"

	"github.com/elastic/beats/v7/x-pack/otel/extension/beatsauthextension"

	// Connectors — forward + agent self-monitoring:
	forwardconnector "go.opentelemetry.io/collector/connector/forwardconnector"

	elasticmonitoringconnector "github.com/elastic/elastic-agent/internal/edot/connectors/elasticmonitoring"

	// Telemetry
	internaltelemetry "github.com/elastic/elastic-agent/internal/edot/internaltelemetry"
	elasticmonitoringreceiver "github.com/elastic/elastic-agent/internal/edot/receivers/elasticmonitoring"
)

// Default returns the factory function for the endpoint-security EDOT collector
// component set. Only beats-based receivers and the components required for
// config translation (see internal/pkg/otel/translate) are included.
// Pass extra extension factories to register them alongside the defaults.
func Default(extensionFactories ...extension.Factory) func() (otelcol.Factories, error) {
	return func() (otelcol.Factories, error) {
		var err error
		factories := otelcol.Factories{
			Telemetry: otelconftelemetry.NewFactory(),
		}

		// Internal telemetry monitoring
		factories.Telemetry = internaltelemetry.NewFactory()

		// Receivers — beats-based inputs + minimal infrastructure
		receivers := []receiver.Factory{
			otlpreceiver.NewFactory(),
			elasticmonitoringreceiver.NewFactory(),
			fbreceiver.NewFactoryWithSettings(fbreceiver.Settings{Home: paths.Components(), Data: paths.Data()}),
			mbreceiver.NewFactoryWithSettings(mbreceiver.Settings{Home: paths.Components(), Data: paths.Data()}),
			nopreceiver.NewFactory(),
		}

		// some receivers should only be available when
		// not in fips mode due to restrictions on crypto usage
		receivers = addNonFipsReceivers(receivers)
		factories.Receivers, err = otelcol.MakeFactoryMap(receivers...)
		if err != nil {
			return otelcol.Factories{}, err
		}

		// Processors — needed for config translation only
		factories.Processors, err = otelcol.MakeFactoryMap[processor.Factory](
			transformprocessor.NewFactory(),
			beatprocessor.NewFactory(),
		)
		if err != nil {
			return otelcol.Factories{}, err
		}

		// Exporters — elasticsearch, logstash, kafka (translate package outputs)
		exporters := []exporter.Factory{
			debugexporter.NewFactory(),
			elasticsearchexporter.NewFactory(),
			nopexporter.NewFactory(),
			logstashexporter.NewFactory(),
		}
		// some exporters should only be available when
		// not in fips mode due to restrictions on crypto usage
		exporters = addNonFipsExporters(exporters)
		factories.Exporters, err = otelcol.MakeFactoryMap(exporters...)
		if err != nil {
			return otelcol.Factories{}, err
		}

		factories.Connectors, err = otelcol.MakeFactoryMap[connector.Factory](
			forwardconnector.NewFactory(),
			elasticmonitoringconnector.NewFactory(),
		)
		if err != nil {
			return otelcol.Factories{}, err
		}

		extensions := []extension.Factory{
			memorylimiterextension.NewFactory(),
			filestorage.NewFactory(),
			healthcheckextension.NewFactory(),
			bearertokenauthextension.NewFactory(),
			pprofextension.NewFactory(),
			beatsauthextension.NewFactory(),
			elasticdiagnostics.NewFactory(),
			elasticsearchstorage.NewFactory(),
			kafkapartitionerextension.NewFactory(),
		}
		extensions = append(extensions, extensionFactories...)
		factories.Extensions, err = otelcol.MakeFactoryMap[extension.Factory](extensions...)
		if err != nil {
			return otelcol.Factories{}, err
		}

		return factories, err
	}
}
