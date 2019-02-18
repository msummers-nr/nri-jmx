package main

import (
	"fmt"
	"io/ioutil"
	"reflect"
	"regexp"
	"testing"

	"github.com/kr/pretty"
	"github.com/newrelic/infra-integrations-sdk/data/metric"
)

func TestJavaAgentParserIsValidFormat(t *testing.T) {
	testCases := []struct {
		file           string
		expectedResult bool
	}{
		{"../test/javaagent-websphere.yml", true},
		{"../test/infra-good.yml", false},
		{"../test/empty.yml", false},
	}
	p := javaAgentJmxParser{}
	for _, tc := range testCases {
		file, _ := ioutil.ReadFile(tc.file)
		r := p.isValidFormat(file)
		if r != tc.expectedResult {
			t.Errorf("Did not get expected result for %s %+v ", tc.file, r)
		}
	}
}

func TestJavaAgentParserParse(t *testing.T) {

}

func TestJavaAgentParserConvertMetricType(t *testing.T) {
	testCases := []struct {
		input  string
		result metric.SourceType
	}{
		{"simple", metric.GAUGE},
		{"Simple", metric.GAUGE},
		{"monotonically_increasing", metric.DELTA},
		{"Monotonically_increasing", metric.DELTA},
		{"attribute", metric.ATTRIBUTE},
		{"Attribute", metric.ATTRIBUTE},
		{"fubar", metric.GAUGE},
	}
	p := javaAgentJmxParser{}
	for _, tc := range testCases {
		r := p.convertMetricType(tc.input)
		if r != tc.result {
			t.Errorf("input: %s expected: %+v received: %+v", tc.input, tc.result, r)
		}
	}
}

func TestJavaAgentParserGetEventType(t *testing.T) {
	testCases := []struct {
		o      string
		d      string
		result string
	}{
		{"oldName", "domainName", "oldName:domainName"},
		{"old Name", "domainName", "old_Name:domainName"},
		{"oldName", "domain Name", "oldName:domain_Name"},
		{"old Name", "domain Name", "old_Name:domain_Name"},
		{"old Name/metric", "domain Name/metric", "old_Name:metric:domain_Name:metric"},
		{"", "domainName", "JMXSample:domainName"},
		{"", "domain Name", "JMXSample:domain_Name"},
		{"", "domain/Name", "JMXSample:domain:Name"},
		{"", "domain Name/metric", "JMXSample:domain_Name:metric"},
		{"", "domain/Name metric", "JMXSample:domain:Name_metric"},
	}
	p := javaAgentJmxParser{}
	for _, tc := range testCases {
		r := p.getEventType(tc.o, tc.d)
		if r != tc.result {
			t.Errorf("old: %s domain: %s expected: %+v received: %+v", tc.o, tc.d, tc.result, r)
		}
	}
}

func TestJavaAgentParserGetMetricName(t *testing.T) {

}

func TestJavaAgentParserMakeInsightCompliant(t *testing.T) {
	testCases := []struct {
		in  string
		out string
	}{
		{"abc", "abc"},
		{"a b c", "a_b_c"},
		{"a/b/c", "a:b:c"},
		{"a:b:c", "a:b:c"},
		{"a_b_c", "a_b_c"},
		{"a b/c", "a_b:c"},
	}
	p := javaAgentJmxParser{}
	for _, tc := range testCases {
		r := p.makeInsightsCompliant(tc.in)
		if r != tc.out {
			t.Errorf("in: %s out: %s received: %+v", tc.in, tc.out, r)
		}
	}
}

func TestJavaAgentParserNormalizeJmxDefinition(t *testing.T) {
	expectedDomains := []*domainDefinition{
		{
			domain:    "WebSphere",
			eventType: "Simple_JMX_File:WebSphere",
			beans: []*beanRequest{
				{
					beanQuery: "type=JDBCProvider,j2eeType=JDBCResource,node=*,process=*,name=*,*",
					attributes: []*attributeRequest{
						{
							attrRegexp: regexp.MustCompile("attr=AllocateCount$"),
							metricName: "AllocateCount",
							metricType: 0,
						},
					},
				},
			},
		},
		{
			domain:    "WebSphere",
			eventType: "Simple_JMX_File:WebSphere",
			beans: []*beanRequest{
				{
					beanQuery: "type=ORB,node=*,process=*,name=*,*",
					attributes: []*attributeRequest{
						{
							attrRegexp: regexp.MustCompile("attr=ConcurrentRequestCount$"),
							metricName: "ConcurrentRequestCount",
							metricType: 0,
						},
						{
							attrRegexp: regexp.MustCompile("attr=LookupTime$"),
							metricName: "LookupTime",
							metricType: 0,
						},
					},
				},
			},
		},
		{
			domain:    "WebSphere",
			eventType: "Simple_JMX_File:WebSphere",
			beans: []*beanRequest{
				{
					beanQuery: "type=DynaCache,process=*,*",
					attributes: []*attributeRequest{
						{
							attrRegexp: regexp.MustCompile("attr=ClientRequestCount$"),
							metricName: "ClientRequestCount",
							metricType: 0,
						},
						{
							attrRegexp: regexp.MustCompile("attr=DependencyIDBasedInvalidationsFromDisk$"),
							metricName: "DependencyIDBasedInvalidationsFromDisk",
							metricType: 0,
						},
					},
				},
			},
		},
	}

	file, err := ioutil.ReadFile("../test/javaagent-simple.yml")
	p := javaAgentJmxParser{}
	d, err := p.parse(file)
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	if !reflect.DeepEqual(d, expectedDomains) {
		fmt.Println(pretty.Diff(d, expectedDomains))
		t.Errorf("Failed to produce expected domains list.")
	}
}

func TestJavaAgentParserParseJavaAgentYaml(t *testing.T) {
	testCases := []struct {
		file         string
		expectedFail bool
	}{
		{"../test/javaagent-websphere.yml", false},
		{"../test/infra-good.yml", true},
		{"../test/empty.yml", true},
	}
	p := javaAgentJmxParser{}
	for _, tc := range testCases {
		file, err := ioutil.ReadFile(tc.file)
		c, err := p.parseJavaAgentYaml(file)
		if (err != nil) && (!tc.expectedFail) {
			t.Errorf("Did not get expected error state for %s %+v %+v ", tc.file, err, c)
		}
	}
}
