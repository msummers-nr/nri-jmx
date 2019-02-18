package main

import (
	"os"
	"regexp"
	"strings"

	"github.com/newrelic/infra-integrations-sdk/data/metric"
	"github.com/newrelic/infra-integrations-sdk/log"
	"gopkg.in/yaml.v2"
)

type javaAgentJmxParser struct {
}

func init() {
	registerParser(javaAgentJmxParser{})
}

func (p javaAgentJmxParser) parse(f []byte) ([]*domainDefinition, error) {
	javaAgentConfig, err := p.parseJavaAgentYaml(f)
	if err != nil {
		log.Error("Failed to parse collection definition file %s: %s", f, err)
		return nil, err
	}

	//reducedDomain, err := reduceJavaAgentYaml(javaAgentConfig)
	//if err != nil {
	//	log.Error("Failed to parse collection definition %s: %s", f, err)
	//	return nil, err
	//}
	//
	//// Validate the definition and create a collection object
	//domain, err := normalizeReducedDefinition(reducedDomain)
	//if err != nil {
	//	log.Error("Failed to parse collection definition %s: %s", f, err)
	//	return nil, err
	//}
	//
	//// Output the new file
	//outputOHIJmxFile(f, domain)

	// Validate the definition and create a collection object
	newCollection, err := p.normalizeJmxDefinition(javaAgentConfig)
	if err != nil {
		log.Error("Failed to parse collection definition %s: %s", f, err)
		os.Exit(1)
	}

	return newCollection, nil
}
func (p javaAgentJmxParser) isValidFormat(f []byte) bool {
	re := regexp.MustCompile(`(?ims)^jmx:(.*?)$`)
	r := re.FindAllStringSubmatch(string(f), -1)
	if len(r) == 1 {
		return true
	}
	return false
}

var defaultEventType = "JMXSample"
var spaceSep = "_"
var metricSep = ":"

// 'Parser' structs for parsing Java Agent YAML format for JMX counters
type javaAgentJmxConfig struct {
	Name    string          `yaml:"name"`
	Version float32         `yaml:"version"`
	Enabled bool            `yaml:"enabled"`
	JMX     []jmxDefinition `yaml:"jmx"`
}

type jmxDefinition struct {
	ObjectName     string             `yaml:"object_name"`
	RootMetricName string             `yaml:"root_metric_name"`
	Metrics        []metricDefinition `yaml:"metrics"`
}

type metricDefinition struct {
	Attributes string `yaml:"attributes"`
	Type       string `yaml:"type"`
}

// 'Reducer' structs for reducing down from old format into minimized version of new format
//type domainReducer struct {
//	EventType string
//	BeansMap  map[string]*beanReducer // String is 'query'
//}

//type beanReducer struct {
//	AttributesMap map[string]*attributeReducer // String is 'attr'
//}
//
//type attributeReducer struct {
//	MetricType string
//	MetricName string // In case we want to have a different metric name than 'attr'
//}

// 'Output' structs for marshaling into yaml output format of nri-jmx

//type collectOutput struct {
//	Collect []*domainOutput `yaml:"collect"`
//}
//
//type domainOutput struct {
//	Domain    string        `yaml:"domain"`
//	EventType string        `yaml:"event_type"`
//	Beans     []*beanOutput `yaml:"beans"`
//}
//
//type beanOutput struct {
//	Query      string             `yaml:"query"`
//	Attributes []*attributeOutput `yaml:"attributes"`
//}
//
//type attributeOutput struct {
//	Attr       string `yaml:"attr"`
//	MetricType string `yaml:"metric_type"`
//	MetricName string `yaml:"metric_name"`
//}

func (p javaAgentJmxParser) parseJavaAgentYaml(f []byte) (*javaAgentJmxConfig, error) {
	var m javaAgentJmxConfig
	if err := yaml.Unmarshal(f, &m); err != nil {
		log.Error("failed to parse collection: %s", err)
		return nil, err
	}
	return &m, nil
}

// Simple Parse: Does not "map/reduce" domains and queries
func (p javaAgentJmxParser) normalizeJmxDefinition(m *javaAgentJmxConfig) ([]*domainDefinition, error) {
	var domains []*domainDefinition

	for _, jmxObject := range m.JMX {
		var domainAndQuery = strings.Split(jmxObject.ObjectName, ":")
		var outbeans []*beanRequest
		for _, thisMetric := range jmxObject.Metrics {
			var inAttrs = strings.Split(thisMetric.Attributes, ",")
			var outAttrs []*attributeRequest
			for _, thisAttr := range inAttrs {
				thisAttr = strings.TrimSpace(thisAttr)
				regex, err := createAttributeRegex(thisAttr, true)
				if err != nil {
					continue
				}
				outAttrs = append(outAttrs, &attributeRequest{attrRegexp: regex, metricType: p.convertMetricType(thisMetric.Type), metricName: p.getMetricName(thisAttr, jmxObject.RootMetricName, domainAndQuery[1])})
			}
			outbeans = append(outbeans, &beanRequest{beanQuery: domainAndQuery[1], attributes: outAttrs})
		}
		domains = append(domains, &domainDefinition{domain: domainAndQuery[0], eventType: p.getEventType(m.Name, domainAndQuery[0]), beans: outbeans})
	}
	return domains, nil
}

//// Reducing Parse: maps, reduces and organizes domains, queries and attributes
//func reduceJavaAgentYaml(m *javaAgentJmxConfig) (map[string]*domainReducer, error) {
//	thisDomainMap := make(map[string]*domainReducer)
//	for _, jmxObject := range m.JMX {
//		var thisDomain *domainReducer
//		var thisBean *beanReducer
//		var domainAndQuery = strings.Split(jmxObject.ObjectName, ":")
//		if _, ok := thisDomainMap[domainAndQuery[0]]; ok {
//			thisDomain = thisDomainMap[domainAndQuery[0]]
//			if _, ok := thisDomain.BeansMap[domainAndQuery[1]]; ok {
//				thisBean = thisDomain.BeansMap[domainAndQuery[1]]
//			}
//		}
//		for _, thisMetric := range jmxObject.Metrics {
//			var inAttrs = strings.Split(thisMetric.Attributes, ",")
//			for _, thisAttr := range inAttrs {
//				thisAttr = strings.TrimSpace(thisAttr)
//				if thisBean != nil {
//					if _, ok := thisBean.AttributesMap[thisAttr]; !ok {
//						thisBean.AttributesMap[thisAttr] = &attributeReducer{MetricType: convertMetricType(thisMetric.Type), MetricName: getMetricName(thisAttr, jmxObject.RootMetricName, domainAndQuery[1])}
//					}
//				} else {
//					thisAttrMap := make(map[string]*attributeReducer)
//					thisAttrMap[thisAttr] = &attributeReducer{MetricType: convertMetricType(thisMetric.Type), MetricName: getMetricName(thisAttr, jmxObject.RootMetricName, domainAndQuery[1])}
//					thisBean = &beanReducer{AttributesMap: thisAttrMap}
//					if thisDomain == nil {
//						var outEventType = getEventType(m.Name, domainAndQuery[0])
//						thisBeanMap := make(map[string]*beanReducer)
//						thisBeanMap[domainAndQuery[1]] = thisBean
//						thisDomainMap[domainAndQuery[0]] = &domainReducer{EventType: outEventType, BeansMap: thisBeanMap}
//					} else {
//						thisDomain.BeansMap[domainAndQuery[1]] = thisBean
//					}
//				}
//			}
//		}
//	}
//	return thisDomainMap, nil
//}
//
//// Builds nri-jmx-compatible yaml from mapped/reduced parse of Java Agent yaml
//func normalizeReducedDefinition(dr map[string]*domainReducer) ([]*domainOutput, error) {
//	var domains []*domainOutput
//	for domain, domainContents := range dr {
//		var beans []*beanOutput
//		for bean, beanContents := range domainContents.BeansMap {
//			var attributes []*attributeOutput
//			for attribute, attributeContents := range beanContents.AttributesMap {
//				attributes = append(attributes, &attributeOutput{Attr: attribute, MetricType: attributeContents.MetricType, MetricName: attributeContents.MetricName})
//			}
//			beans = append(beans, &beanOutput{Query: bean, Attributes: attributes})
//		}
//		domains = append(domains, &domainOutput{Domain: domain, EventType: domainContents.EventType, Beans: beans})
//	}
//	return domains, nil
//}

//// Spits out the nri-jmx-compatible yaml file
//func outputOHIJmxFile(filename string, d []*domainOutput) {
//	log.Info("New File: " + filename + ".new\n")
//	m, err := yaml.Marshal(&collectOutput{Collect: d})
//	if err != nil {
//		fmt.Printf("error: %v", err)
//	}
//	fmt.Printf("%s", string(m))
//}

func (p javaAgentJmxParser) convertMetricType(metrictype string) metric.SourceType {
	switch strings.TrimSpace(strings.ToLower(metrictype)) {
	case "simple":
		return metric.GAUGE
	case "monotonically_increasing":
		return metric.DELTA
	case "attribute":
		return metric.ATTRIBUTE
	default:
		return metric.GAUGE
	}
}

func (p javaAgentJmxParser) getEventType(oldName string, domainName string) string {
	if oldName == "" {
		return p.makeInsightsCompliant(defaultEventType + metricSep + domainName)
	}
	return p.makeInsightsCompliant(oldName + metricSep + domainName)
}

func (p javaAgentJmxParser) getMetricName(attrName string, rootMetricName string, query string) string {
	if rootMetricName == "" {
		return attrName
	}

	objNameRegex, _ := regexp.Compile("{\\w+}")
	if objNameRegex.MatchString(rootMetricName) {
		var queryStrings = strings.Split(query, ",")
		queryMap := make(map[string]string)
		for _, thisQuery := range queryStrings {
			var querySplit = strings.Split(thisQuery, "=")
			queryMap[querySplit[0]] = querySplit[1]
		}
		var matchedObjs = objNameRegex.FindAllString(rootMetricName, -1)
		for _, thisObj := range matchedObjs {
			var testObj = thisObj[1 : len(thisObj)-1]
			if objVal, ok := queryMap[testObj]; ok {
				rootMetricName = strings.Replace(rootMetricName, thisObj, objVal, -1)
			}
		}
	}
	return p.makeInsightsCompliant(rootMetricName + metricSep + attrName)
}

func (p javaAgentJmxParser) makeInsightsCompliant(inString string) string {
	inString = strings.Replace(inString, " ", spaceSep, -1)
	inString = strings.Replace(inString, "/", metricSep, -1)
	return inString
}
