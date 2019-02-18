package main

import (
	"errors"
	"regexp"

	"github.com/newrelic/infra-integrations-sdk/data/metric"
)

type parser interface {
	parse(f []byte) ([]*domainDefinition, error)
	isValidFormat(f []byte) bool
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

// Parsers must self register, init() is good for that
func registerParser(p parser) {
	parsers = append(parsers, p)
}

func getParser(f []byte) (parser, error) {
	for _, reader := range parsers {
		if reader.isValidFormat(f) {
			return reader, nil
		}
	}
	return nil, errors.New("No valid parser found for JMX file")
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
