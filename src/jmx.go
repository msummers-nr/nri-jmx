package main

import (
	"io/ioutil"
	"os"
	"strings"

	sdkArgs "github.com/newrelic/infra-integrations-sdk/args"
	"github.com/newrelic/infra-integrations-sdk/integration"
	"github.com/newrelic/infra-integrations-sdk/jmx"
	"github.com/newrelic/infra-integrations-sdk/log"
)

type argumentList struct {
	sdkArgs.DefaultArgumentList
	JmxHost            string `default:"localhost" help:"The host running JMX"`
	JmxPort            string `default:"9999" help:"The port JMX is running on"`
	JmxUser            string `default:"admin" help:"The username for the JMX connection"`
	JmxPass            string `default:"admin" help:"The password for the JMX connection"`
	JmxRemote          bool   `default:"false" help:"When activated uses the JMX remote url connection format"`
	KeyStore           string `default:"" help:"The location for the keystore containing JMX Client's SSL certificate"`
	KeyStorePassword   string `default:"" help:"Password for the SSL Key Store"`
	TrustStore         string `default:"" help:"The location for the keystore containing JMX Server's SSL certificate"`
	TrustStorePassword string `default:"" help:"Password for the SSL Trust Store"`
	CollectionFiles    string `default:"" help:"A comma separated list of full paths to metrics configuration files"`
	Timeout            int    `default:"10000" help:"Timeout for JMX queries"`
	MetricLimit        int    `default:"200" help:"Number of metrics that can be collected per entity. If this limit is exceeded the entity will not be reported. A limit of 0 implies no limit."`
}

const (
	integrationName    = "com.newrelic.jmx"
	integrationVersion = "1.0.4"
)

var (
	args argumentList

	// Alias these functions for easy mocking
	jmxOpen  = jmx.Open
	jmxClose = jmx.Close
	jmxQuery = jmx.Query
)

func main() {

	// Create a new integration
	jmxIntegration, err := integration.New(integrationName, integrationVersion, integration.Args(&args))
	if err != nil {
		os.Exit(1)
	}
	log.SetupLogging(args.Verbose)

	//<<<<<<< HEAD
	//	// Ensure a collection file is specified
	//	if args.CollectionFiles == "" {
	//		log.Error("Must specify at least one configuration file")
	//		os.Exit(1)
	//	}
	//
	//	// Open a JMX connection
	//	if err := jmxOpen(args.JmxHost, args.JmxPort, args.JmxUser, args.JmxPass); err != nil {
	//=======
	options := make([]jmx.Option, 0)
	if args.JmxRemote {
		options = append(options, jmx.WithRemoteProtocol())
	}
	if args.KeyStore != "" && args.KeyStorePassword != "" && args.TrustStore != "" && args.TrustStorePassword != "" {
		ssl := jmx.WithSSL(args.KeyStore, args.KeyStorePassword, args.TrustStore, args.TrustStorePassword)
		options = append(options, ssl)
	}
	if err := jmxOpen(args.JmxHost, args.JmxPort, args.JmxUser, args.JmxPass, options...); err != nil {
		//>>>>>>> upstream/master
		log.Error(
			"Failed to open JMX connection (host: %s, port: %s, user: %s, pass: %s, keyStore: %s, keyStorePassword: %s, trustStore: %s, trustStorePassword: %s, remote: %t): %s",
			args.JmxHost, args.JmxPort, args.JmxUser, args.JmxPass, args.KeyStore, args.KeyStorePassword, args.TrustStore, args.TrustStorePassword, args.JmxRemote, err,
		)
		os.Exit(1)
	}

	for _, f := range strings.Split(args.CollectionFiles, ",") {
		file, err := ioutil.ReadFile(f)
		if err != nil {
			log.Error("Error reading JMX configuration file: %s error: %+v", f, err)
			continue
		}

		p, err := getParser(file)
		if err != nil {
			log.Error("Error getting JMX parser for file: %s error: %+v", f, err)
			continue
		}

		d, err := p.parse(file)
		if err != nil {
			log.Error("Error parsing JMX file: %s error: %+v", f, err)
			continue
		}

		err = queryJMX(d, jmxIntegration)
		if err != nil {
			log.Error("Failed to process domainDefinition: %s", err)
		}
	}

	jmxClose()
	jmxIntegration.Entities = checkMetricLimit(jmxIntegration.Entities)
	if err := jmxIntegration.Publish(); err != nil {
		log.Error("Failed to publish integration: %s", err.Error())
		os.Exit(1)
	}
}

// checkMetricLimit looks through all of the metric sets for every entity and aggregates the number
// of metrics. If that total is greate than args.MetricLimit a warning is logged
func checkMetricLimit(entities []*integration.Entity) []*integration.Entity {
	validEntities := make([]*integration.Entity, 0, len(entities))

	for _, entity := range entities {
		metricCount := 0
		for _, metricSet := range entity.Metrics {
			metricCount += len(metricSet.Metrics)
		}

		if args.MetricLimit != 0 && metricCount > args.MetricLimit {
			log.Warn("Domain '%s' has %d metrics, the current limit is %d. This Domain will not be reported", entity.Metadata.Name, metricCount, args.MetricLimit)
			continue
		}

		validEntities = append(validEntities, entity)
	}

	return validEntities
}
