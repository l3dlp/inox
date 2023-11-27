package project_server

type IndividualServerConfig struct {
	MaxWebSocketPerIp      int    `json:"maxWebsocketPerIp,omitempty"`
	IgnoreInstalledBrowser bool   `json:"ignoreInstalledBrowser,omitempty"`
	ProjectsDir            string `json:"projectsDir,omitempty"` //if not set, defaults to filepath.Join(config.USER_HOME, "inox-projects")
	BehindCloudProxy       bool   `json:"behindCloudProxy,omitempty"`
	Port                   int    `json:"port,omitempty"`
	BindToAllInterfaces    bool   `json:"bindToAllInterfaces,omitempty"`
	AllowBrowserAutomation bool   `json:"allowBrowserAutomation,omitempty"`
}
