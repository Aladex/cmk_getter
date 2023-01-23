package utils

import (
	"cmk_getter/config"
	"time"
)

const cmkVersionUrl = "version"

var cmkSite = config.ConfigCmkGetter.Site
var cmkDomain = config.ConfigCmkGetter.Domain

const urlTemplate = "https://%s/%s/check_mk/api/1.0/%s"
const downloadUrlTemplate = "check_mk/api/1.0/domain-types/agent/actions/download/invoke?os_type=linux_deb"
const hostConfigUrl = "check_mk/api/1.0/domain-types/host_config/collections/all"

var CurrentVersion string = ""

type CmkVersionChanges struct {
	Version         string `json:"version"`
	ErrorString     string `json:"error_string"`
	TriggerDownload bool   `json:"trigger_download"`
	Folder          string `json:"folder"`
}

type CmkVersionResponse struct {
	Site    string `json:"site"`
	Group   string `json:"group"`
	RestApi struct {
		Revision string `json:"revision"`
	} `json:"rest_api"`
	Versions struct {
		Apache  []int  `json:"apache"`
		Checkmk string `json:"checkmk"`
		Python  string `json:"python"`
		ModWsgi []int  `json:"mod_wsgi"`
		Wsgi    []int  `json:"wsgi"`
	} `json:"versions"`
	Edition string `json:"edition"`
	Demo    bool   `json:"demo"`
}

// CmkHostConfigResponse is the response from the check_mk API for the host config
type CmkHostConfigResponse struct {
	Links []struct {
		DomainType string `json:"domainType"`
		Rel        string `json:"rel"`
		Href       string `json:"href"`
		Method     string `json:"method"`
		Type       string `json:"type"`
	} `json:"links"`
	Id         string `json:"id"`
	DomainType string `json:"domainType"`
	Value      []struct {
		Links []struct {
			DomainType string `json:"domainType"`
			Rel        string `json:"rel"`
			Href       string `json:"href"`
			Method     string `json:"method"`
			Type       string `json:"type"`
			Title      string `json:"title,omitempty"`
		} `json:"links"`
		DomainType string `json:"domainType"`
		Id         string `json:"id"`
		Title      string `json:"title"`
		Members    struct {
		} `json:"members"`
		Extensions struct {
			Folder     string `json:"folder"`
			Attributes struct {
				Alias     string `json:"alias,omitempty"`
				Ipaddress string `json:"ipaddress,omitempty"`
				MetaData  struct {
					CreatedAt time.Time `json:"created_at"`
					UpdatedAt time.Time `json:"updated_at"`
					CreatedBy string    `json:"created_by"`
				} `json:"meta_data"`
				TagCriticality   string `json:"tag_criticality,omitempty"`
				TagAgent         string `json:"tag_agent,omitempty"`
				TagAddressFamily string `json:"tag_address_family,omitempty"`
				TagSnmpDs        string `json:"tag_snmp_ds,omitempty"`
				TagNetworking    string `json:"tag_networking,omitempty"`
				SnmpCommunity    struct {
					Type            string `json:"type"`
					Community       string `json:"community,omitempty"`
					AuthProtocol    string `json:"auth_protocol,omitempty"`
					SecurityName    string `json:"security_name,omitempty"`
					AuthPassword    string `json:"auth_password,omitempty"`
					PrivacyProtocol string `json:"privacy_protocol,omitempty"`
					PrivacyPassword string `json:"privacy_password,omitempty"`
				} `json:"snmp_community,omitempty"`
				TagCheckMkAgentConn string `json:"tag_check_mk-agent-conn,omitempty"`
				Labels              struct {
					Purpose     string `json:"purpose,omitempty"`
					CmkOsFamily string `json:"cmk/os_family,omitempty"`
				} `json:"labels,omitempty"`
				TagPiggyback string `json:"tag_piggyback,omitempty"`
			} `json:"attributes"`
			EffectiveAttributes interface{} `json:"effective_attributes"`
			IsCluster           bool        `json:"is_cluster"`
			IsOffline           bool        `json:"is_offline"`
			ClusterNodes        interface{} `json:"cluster_nodes"`
		} `json:"extensions"`
	} `json:"value"`
}
