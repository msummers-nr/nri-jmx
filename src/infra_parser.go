package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"

	"github.com/newrelic/infra-integrations-sdk/data/metric"
	"github.com/newrelic/infra-integrations-sdk/log"
	yaml "gopkg.in/yaml.v2"
)

type infraJmxParser struct {
}

func (p infraJmxParser) parse(f string) ([]*domainDefinition, error) {
	// Check that the filepath is an absolute path
	if !filepath.IsAbs(f) {
		log.Error("Invalid configuration file path %s. JMX configuration files must be specified as absolute paths.", f)
		os.Exit(1)
	}

	// Parse the yaml file into a raw definition
	collectionDefinition, err := parseYaml(f)
	if err != nil {
		log.Error("Failed to parse configuration file %s: %s", f, err)
		return nil, err
	}

	// Validate the definition and create a domainDefinition object
	domainDefinition, err := parseCollectionDefinition(collectionDefinition)
	if err != nil {
		log.Error("Failed to parse domainDefinition definition %s: %s", f, err)
		return nil, err
	}

	return domainDefinition, nil
}
func (p infraJmxParser) isValidFormat(f string) bool {
	// TODO
	return true
}

func init() {
	addParser(infraJmxParser{})
}

// collectionDefinition is a struct to aid the automatic
// parsing of a collection yaml file
type collectionDefinition struct {
	Collect []struct {
		Domain    string           `yaml:"domain"`
		EventType string           `yaml:"event_type"`
		Beans     []beanDefinition `yaml:"beans"`
	}
}

// beanDefinition is a struct to aid the automatic
// parsing of a collection yaml file
type beanDefinition struct {
	Query      string        `yaml:"query"`
	Exclude    interface{}   `yaml:"exclude_regex"`
	Attributes []interface{} `yaml:"attributes"`
}

var (
	// metricTypes maps the string used in yaml to a metric type
	metricTypes = map[string]metric.SourceType{
		"gauge":     metric.GAUGE,
		"delta":     metric.DELTA,
		"attribute": metric.ATTRIBUTE,
		"rate":      metric.RATE,
	}
)

// parseYaml reads a yaml file and parses it into a collectionDefinition.
// It validates syntax only and not content
func parseYaml(filename string) (*collectionDefinition, error) {
	// Read the file
	yamlFile, err := ioutil.ReadFile(filename)
	if err != nil {
		log.Error("failed to open %s: %s", filename, err)
		return nil, err
	}

	// Parse the file
	var c collectionDefinition
	if err := yaml.Unmarshal(yamlFile, &c); err != nil {
		log.Error("failed to parse collection: %s", err)
		return nil, err
	}

	return &c, nil
}

// parseCollection takes a raw collectionDefinition and returns
// an array of domains containing the validated configuration
func parseCollectionDefinition(c *collectionDefinition) ([]*domainDefinition, error) {

	var err error

	// For each domain in the collection
	var collections []*domainDefinition
	for _, domain := range c.Collect {

		// For each bean in the domain
		var beans []*beanRequest
		var newBean *beanRequest
		for _, bean := range domain.Beans {

			// Parse the bean and add it to the domain
			newBean, err = parseBean(&bean)
			if err != nil {
				return nil, err
			}

			beans = append(beans, newBean)
		}

		// If no custom event type defined, generate an event type from the domain name
		var eventType string
		if domain.EventType == "" {
			eventType, err = generateEventType(domain.Domain)
			if err != nil {
				return nil, err
			}
		} else {
			eventType = domain.EventType
		}
		collections = append(collections, &domainDefinition{domain: domain.Domain, eventType: eventType, beans: beans})
	}

	return collections, nil
}

func parseBean(bean *beanDefinition) (*beanRequest, error) {
	attributes, err := parseAttributes(bean.Attributes)
	if err != nil {
		return nil, err
	}

	// Parse the exclude patterns
	var excludePatterns []*regexp.Regexp
	if bean.Exclude != nil {
		switch b := bean.Exclude.(type) {
		// If exclude_regex is a string
		case string:
			r, err := regexp.Compile(b)
			if err != nil {
				return nil, fmt.Errorf("invalid regex pattern %s", b)
			}
			excludePatterns = append(excludePatterns, r)
		// If exclude_regex is an array of strings
		case []interface{}:
			for _, excludeString := range b {
				switch e := excludeString.(type) {
				case string:
					// Panic if can't compile the regex pattern
					r := regexp.MustCompile(e)
					excludePatterns = append(excludePatterns, r)
				default:
					return nil, fmt.Errorf("invalid exclude pattern '%v'", e)
				}
			}
		default:
			return nil, fmt.Errorf("invalid format for exclude_regex")

		}
	}

	return &beanRequest{beanQuery: bean.Query, exclude: excludePatterns, attributes: attributes}, nil
}

func parseAttributes(rawAttributes []interface{}) ([]*attributeRequest, error) {
	var attributes []*attributeRequest
	// If no attributes are specified, collect all
	if len(rawAttributes) == 0 {
		// We know this is valid regex, so we don't need to handle the error
		r, _ := createAttributeRegex(".*", false)
		attributes = []*attributeRequest{
			{
				attrRegexp: r,
				metricType: -1,
			},
		}
	} else {
		// For each defined attribute
		for _, attribute := range rawAttributes {
			var newAttribute *attributeRequest
			var err error
			switch a := attribute.(type) {
			// If it's a map
			case map[interface{}]interface{}:
				newAttribute, err = parseAttributeFromMap(a)
			// If it's only the attribute name
			case string:
				newAttribute, err = parseAttributeFromString(a)
			default:
				return nil, fmt.Errorf("unable to parse attributes list %s", attribute)
			}
			if err != nil {
				return nil, err
			}
			attributes = append(attributes, newAttribute)
		}
	}

	return attributes, nil
}

func parseAttributeFromString(a string) (*attributeRequest, error) {
	attrRegexp, err := createAttributeRegex(a, true)
	if err != nil {
		return nil, fmt.Errorf("failed to create regex pattern from attribute name %s", a)
	}

	return &attributeRequest{attrRegexp: attrRegexp, metricType: -1}, nil
}

func parseAttributeFromMap(a map[interface{}]interface{}) (*attributeRequest, error) {
	attrName, namePresent := a["attr"]
	attrRegexpString, regexPresent := a["attr_regex"]
	var attrRegexp *regexp.Regexp
	var err error

	// Must specify exactly one attribute selector
	if namePresent == regexPresent {
		return nil, fmt.Errorf("must specify one of attr or attr_regex for every attribute")
	}

	if regexPresent {
		attrRegexp, err = createAttributeRegex(attrRegexpString.(string), false)
		if err != nil {
			return nil, fmt.Errorf("failed to compile attribute regex pattern %s", attrRegexpString)
		}
	} else {
		attrRegexp, err = createAttributeRegex(attrName.(string), true)
		if err != nil {
			return nil, fmt.Errorf("failed to create regex pattern from attribute name %s", attrName.(string))
		}
	}

	// Parse the metric type
	metricType, err := getMetricType(a)
	if err != nil {
		return nil, err
	}

	newAttribute := &attributeRequest{
		attrRegexp: attrRegexp,
		metricType: metricType,
	}

	// Parse the metric name
	metricName, _ := a["metric_name"]
	if metricName != nil {
		newAttribute.metricName = metricName.(string)
	}

	return newAttribute, nil

}

func getMetricType(a map[interface{}]interface{}) (metric.SourceType, error) {
	metricTypeString, ok := a["metric_type"]
	var metricType metric.SourceType
	if !ok {
		metricType = -1 // Since metric type can't be nil, using -1 as a placeholder
	} else {
		mt, ok := metricTypes[metricTypeString.(string)]
		if !ok {
			return 0, fmt.Errorf("invalid metric type %s", metricTypeString.(string))
		}
		metricType = mt
	}

	return metricType, nil
}
