package main

import (
	"errors"
	"regexp"

	"github.com/newrelic/infra-integrations-sdk/data/metric"
)

type parser interface {
	parse(f string) ([]*domainDefinition, error)
	isValidFormat(f string) bool
}

// domainDefinition is a validated and simplified
// representation of the requested collection parameters
// from a single domain
type domainDefinition struct {
	domain    string
	eventType string
	beans     []*beanRequest
}

// attributeRequest is a storage struct containing
// the information necessary to turn a JMX attribute
// into a metric
type attributeRequest struct {
	// attrRegexp is a compiled regex pattern that matches the attribute
	attrRegexp *regexp.Regexp
	metricName string
	metricType metric.SourceType
}

// beanRequest is a storage struct containing the
// information necessary to query a JMX endpoint
// and filter the results
type beanRequest struct {
	beanQuery string
	// exclude is a list of compiled regex that matches beans to exclude from collection
	exclude    []*regexp.Regexp
	attributes []*attributeRequest
}

var parsers []parser

func addParser(p parser) {
	parsers = append(parsers, p)
}

func getParser(filename string) (parser, error) {
	for _, reader := range parsers {
		if reader.isValidFormat(filename) {
			return reader, nil
		}
	}
	return nil, errors.New("No valid parser found for JMX file: " + filename)
}
func createAttributeRegex(attrRegex string, literal bool) (*regexp.Regexp, error) {
	var attrString string
	// If attrRegex is the actual attribute name, and not a regex match
	if literal {
		attrString = regexp.QuoteMeta(attrRegex)
	} else {
		attrString = attrRegex
	}

	r, err := regexp.Compile("attr=" + attrString + "$")
	if err != nil {
		return nil, err
	}

	return r, nil
}
