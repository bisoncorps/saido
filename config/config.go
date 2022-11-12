package config

import (
	"fmt"
	"io/ioutil"

	"github.com/bisohns/saido/driver"
	"github.com/bisohns/saido/inspector"
	"github.com/mitchellh/mapstructure"
	log "github.com/sirupsen/logrus"

	"gopkg.in/yaml.v2"
)

type DashboardInfo struct {
	Hosts        []Host
	Metrics      map[string]string
	Title        string
	PollInterval int
}

type HostList = []string

func Contains(hostList HostList, host Host) bool {
	for _, compare := range hostList {
		if host.Address == compare {
			return true
		}
	}
	return false
}

// GetAllHostAddresses : returns list of all hosts in the dashboard
func (dashboardInfo *DashboardInfo) GetAllHostAddresses() (addresses HostList) {
	addresses = []string{}
	for _, host := range dashboardInfo.Hosts {
		addresses = append(addresses, host.Address)
	}
	return
}

type Connection struct {
	Type                 string `mapstructure:"type"`
	Username             string `mapstructure:"username"`
	Password             string `mapstructure:"password"`
	PrivateKeyPath       string `mapstructure:"private_key_path"`
	PrivateKeyPassPhrase string `mapstructure:"private_key_passphrase"`
	Port                 int32  `mapstructure:"port"`
	Host                 string
}

func (conn *Connection) ToDriver() driver.Driver {
	switch conn.Type {
	case "ssh":
		return &driver.SSH{
			User:            conn.Username,
			Host:            conn.Host,
			Port:            int(conn.Port),
			KeyFile:         conn.PrivateKeyPath,
			KeyPass:         conn.PrivateKeyPassPhrase,
			Password:        conn.Password,
			CheckKnownHosts: false,
		}
	default:
		return &driver.Local{}
	}
}

type Host struct {
	Address    string
	Alias      string
	Connection *Connection
}

type Config struct {
	Hosts        map[interface{}]interface{} `yaml:"hosts"`
	Metrics      map[interface{}]interface{} `yaml:"metrics"`
	Title        string                      `yaml:"title"`
	PollInterval int                         `yaml:"poll-interval"`
}

func LoadConfig(configPath string) *Config {
	var config = &Config{}
	confYaml, err := ioutil.ReadFile(configPath)
	if err != nil {
		log.Errorf("yamlFile.Get err   %v ", err)
	}
	err = yaml.Unmarshal([]byte(confYaml), &config)
	if err != nil {
		log.Fatalf("error: %v", err)
	}
	return config
}

func GetDashboardInfoConfig(config *Config) *DashboardInfo {
	dashboardInfo := &DashboardInfo{
		Title: "Saido",
	}
	if config.Title != "" {
		dashboardInfo.Title = config.Title
	}
	metrics := make(map[string]string)

	dashboardInfo.Hosts = parseConfig("root", "", config.Hosts, &Connection{})
	for metric, customCommand := range config.Metrics {
		metric := fmt.Sprintf("%v", metric)
		if inspector.Valid(metric) {
			metrics[metric] = fmt.Sprintf("%v", customCommand)
		} else {
			log.Fatalf("Found invalid metric %v", metric)
		}
	}
	dashboardInfo.Metrics = metrics
	for _, host := range dashboardInfo.Hosts {
		log.Debugf("%s: %v", host.Address, host.Connection)
	}
	if config.PollInterval < 5 {
		log.Fatal("Cannot set poll interval below 5 seconds")
	}
	dashboardInfo.PollInterval = config.PollInterval
	return dashboardInfo
}

func parseConnection(conn map[interface{}]interface{}) *Connection {
	var c Connection
	mapstructure.Decode(conn, &c)
	if c.Type == "ssh" && c.Port == 0 {
		c.Port = 22
	}
	if c.Password != "" && c.PrivateKeyPath != "" {
		log.Fatal("Cannot specify both password login and private key login on same connection")
	}
	return &c
}

func parseConfig(name string, host string, group map[interface{}]interface{}, currentConnection *Connection) []Host {
	currentConn := currentConnection
	allHosts := []Host{}
	log.Infof("Loading config for %s and host: %s with Connection: %+v", name, host, currentConn)
	isParent := false // Set to true for groups that contain just children data i.e children
	if conn, ok := group["connection"]; ok {
		v, ok := conn.(map[interface{}]interface{})
		if !ok {
			log.Errorf("Failed to parse connection for %s", name)
		}

		currentConn = parseConnection(v)
	}

	if children, ok := group["children"]; ok {
		isParent = true
		parsedChildren, ok := children.(map[interface{}]interface{})
		if !ok {
			log.Fatalf("Failed to parse children of %s", name)
			return nil
		}

		for k, v := range parsedChildren {
			host := make(map[interface{}]interface{})
			host, ok := v.(map[interface{}]interface{})
			if !ok && v != nil { // some leaf nodes do not contain extra data under
				log.Errorf("Faled to parse children of %s", name)
			}
			allHosts = append(allHosts, parseConfig(fmt.Sprintf("%s:%s", name, k), fmt.Sprintf("%s", k), host, currentConn)...)
		}
	}

	if !isParent {
		currentConn.Host = host
		newHost := Host{
			Address:    host,
			Connection: currentConn,
		}
		if alias, ok := group["alias"]; ok {
			newHost.Alias = alias.(string)
		}

		allHosts = append(allHosts, newHost)
	}
	return allHosts
}
