// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package otelcol

import (
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/otelcol"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/service/telemetry/otelconftelemetry"

	"github.com/elastic/elastic-agent/internal/pkg/agent/application/paths"

	// Receivers:

	// for collecting log files

	// for collecting APM data from Elastic APM agents

	fbreceiver "github.com/elastic/beats/v7/x-pack/filebeat/fbreceiver"

	// Processors:
	// for modifying signal attributes

	// for adding geographical metadata associated to an IP address
	// for adding k8s metadata
	// for deduplicating log events

	// for modifying resource attributes
	// for tail-based sampling
	transformprocessor "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor" // for OTTL processing on logs
	// for batching events

	"github.com/elastic/opentelemetry-collector-components/processor/ratelimitprocessor"

	// Exporters:
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter" // for e2e tests
	// for dev
	"go.opentelemetry.io/collector/exporter/otlpexporter"

	"github.com/elastic/beats/v7/x-pack/otel/processor/beatprocessor"

	// Extensions

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/cgroupruntimeextension"
	headersetterextension "github.com/open-telemetry/opentelemetry-collector-contrib/extension/headerssetterextension"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextension"
	healthcheckv2extension "github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckv2extension"
	opampextension "github.com/open-telemetry/opentelemetry-collector-contrib/extension/opampextension"
	pprofextension "github.com/open-telemetry/opentelemetry-collector-contrib/extension/pprofextension"
	"go.opentelemetry.io/collector/extension/memorylimiterextension" // for putting backpressure when approach a memory limit

	elasticsearchstorage "github.com/elastic/beats/v7/x-pack/otel/extension/elasticsearchstorage"
	elasticdiagnostics "github.com/elastic/elastic-agent/internal/pkg/otel/extension/elasticdiagnostics"

	// Connectors

	forwardconnector "go.opentelemetry.io/collector/connector/forwardconnector"

	"github.com/elastic/beats/v7/x-pack/otel/extension/beatsauthextension"

	// Telemetry
	internaltelemetry "github.com/elastic/elastic-agent/internal/edot/internaltelemetry"
)

func components(extensionFactories ...extension.Factory) func() (otelcol.Factories, error) {
	return func() (otelcol.Factories, error) {
		var err error
		factories := otelcol.Factories{
			Telemetry: otelconftelemetry.NewFactory(),
		}

		// Internal telemetry monitoring
		factories.Telemetry = internaltelemetry.NewFactory()

		// Receivers
		receivers := []receiver.Factory{
			// dockerstatsreceiver.NewFactory(),
			// elasticapmintakereceiver.NewFactory(),
			// otlpreceiver.NewFactory(),
			// filelogreceiver.NewFactory(),
			// kubeletstatsreceiver.NewFactory(),
			// k8sclusterreceiver.NewFactory(),
			// k8seventsreceiver.NewFactory(),
			// hostmetricsreceiver.NewFactory(),
			// httpcheckreceiver.NewFactory(),
			// k8sobjectsreceiver.NewFactory(),
			// receivercreator.NewFactory(),
			// redisreceiver.NewFactory(),
			// nginxreceiver.NewFactory(),
			// jaegerreceiver.NewFactory(),
			// zipkinreceiver.NewFactory(),
			// elasticmonitoringreceiver.NewFactory(),
			// verifierreceiver.NewFactory(),
			fbreceiver.NewFactoryWithSettings(fbreceiver.Settings{Home: paths.Components(), Data: paths.Data()}),
			// mbreceiver.NewFactoryWithSettings(mbreceiver.Settings{Home: paths.Components(), Data: paths.Data()}),
			// nopreceiver.NewFactory(),
			// apachereceiver.NewFactory(),
			// couchdbreceiver.NewFactory(),
			// haproxyreceiver.NewFactory(),
			// iisreceiver.NewFactory(),
			// memcachedreceiver.NewFactory(),
			// mongodbreceiver.NewFactory(),
			// mysqlreceiver.NewFactory(),
			// oracledbreceiver.NewFactory(),
			// postgresqlreceiver.NewFactory(),
			// rabbitmqreceiver.NewFactory(),
			// snmpreceiver.NewFactory(),
			// kafkametricsreceiver.NewFactory(),
			// sqlserverreceiver.NewFactory(),
			// statsdreceiver.NewFactory(),
			// vcenterreceiver.NewFactory(),
			// zookeeperreceiver.NewFactory(),
			// windowseventlogreceiver.NewFactory(),
			// awss3receiver.NewFactory(),
			// windowsperfcountersreceiver.NewFactory(),
			// prometheusremotewritereceiver.NewFactory(),
		}

		// some receivers are only available on certain OS.
		// receivers = addOsSpecificReceivers(receivers)

		// some receivers should only be available when
		// not in fips mode due to restrictions on crypto usage
		// receivers = addNonFipsReceivers(receivers)
		factories.Receivers, err = otelcol.MakeFactoryMap(receivers...)
		if err != nil {
			return otelcol.Factories{}, err
		}

		// Processors
		factories.Processors, err = otelcol.MakeFactoryMap[processor.Factory](
			// batchprocessor.NewFactory(),
			// resourceprocessor.NewFactory(),
			// attributesprocessor.NewFactory(),
			// cumulativetodeltaprocessor.NewFactory(),
			transformprocessor.NewFactory(),
			// filterprocessor.NewFactory(),
			// geoipprocessor.NewFactory(),
			// k8sattributesprocessor.NewFactory(),
			// elasticinframetricsprocessor.NewFactory(),
			// resourcedetectionprocessor.NewFactory(),
			// memorylimiterprocessor.NewFactory(),
			// elasticapmprocessor.NewFactory(),
			// elastictraceprocessor.NewFactory(), // deprecated, will be removed in future
			ratelimitprocessor.NewFactory(),
			// tailsamplingprocessor.NewFactory(),
			// logdedupprocessor.NewFactory(),
			beatprocessor.NewFactory(),
		)
		if err != nil {
			return otelcol.Factories{}, err
		}

		// Exporters
		exporters := []exporter.Factory{
			otlpexporter.NewFactory(),
			// debugexporter.NewFactory(),
			// fileexporter.NewFactory(),
			elasticsearchexporter.NewFactory(),
			// loadbalancingexporter.NewFactory(),
			// otlphttpexporter.NewFactory(),
			// nopexporter.NewFactory(),
			// logstashexporter.NewFactory(),
		}
		// some exporters should only be available when
		// not in fips mode due to restrictions on crypto usage
		// exporters = addNonFipsExporters(exporters)
		factories.Exporters, err = otelcol.MakeFactoryMap(exporters...)
		if err != nil {
			return otelcol.Factories{}, err
		}

		factories.Connectors, err = otelcol.MakeFactoryMap[connector.Factory](
			// otlpjsonconnector.NewFactory(),
			// routingconnector.NewFactory(),
			// spanmetricsconnector.NewFactory(),
			// elasticapmconnector.NewFactory(),
			// profilingmetricsconnector.NewFactory(),
			forwardconnector.NewFactory(),
		)
		if err != nil {
			return otelcol.Factories{}, err
		}

		extensions := []extension.Factory{
			cgroupruntimeextension.NewFactory(),
			// k8sleaderelector.NewFactory(),
			healthcheckv2extension.NewFactory(),
			memorylimiterextension.NewFactory(),
			// filestorage.NewFactory(),
			healthcheckextension.NewFactory(),
			// bearertokenauthextension.NewFactory(),
			pprofextension.NewFactory(),
			// k8sobserver.NewFactory(),
			// apikeyauthextension.NewFactory(),
			// apmconfigextension.NewFactory(),
			headersetterextension.NewFactory(),
			beatsauthextension.NewFactory(),
			elasticdiagnostics.NewFactory(),
			elasticsearchstorage.NewFactory(),
			// awslogsencodingextension.NewFactory(),
			opampextension.NewFactory(),
		}
		extensions = append(extensions, extensionFactories...)
		factories.Extensions, err = otelcol.MakeFactoryMap[extension.Factory](extensions...)
		if err != nil {
			return otelcol.Factories{}, err
		}

		return factories, err
	}
}
