package domain

type TLSOptions struct {
	Enabled            bool            `json:"enabled" yaml:"enabled"`
	ServerName         string          `json:"server_name,omitempty" yaml:"server_name,omitempty"`
	InsecureSkipVerify bool            `json:"insecure_skip_verify,omitempty" yaml:"insecure_skip_verify,omitempty"`
	ALPN               []string        `json:"alpn,omitempty" yaml:"alpn,omitempty"`
	ClientFingerprint  string          `json:"client_fingerprint,omitempty" yaml:"client_fingerprint,omitempty"`
	Fingerprint        string          `json:"fingerprint,omitempty" yaml:"fingerprint,omitempty"`
	Certificate        string          `json:"certificate,omitempty" yaml:"certificate,omitempty"`
	PrivateKey         string          `json:"private_key,omitempty" yaml:"private_key,omitempty"`
	DisableSNI         bool            `json:"disable_sni,omitempty" yaml:"disable_sni,omitempty"`
	ECH                *ECHOptions     `json:"ech,omitempty" yaml:"ech,omitempty"`
	Reality            *RealityOptions `json:"reality,omitempty" yaml:"reality,omitempty"`
}

type DialerOptions struct {
	Network  string `json:"network,omitempty" yaml:"network,omitempty"`
	TFO      bool   `json:"tfo,omitempty" yaml:"tfo,omitempty"`
	UDPRelay *bool  `json:"udp_relay,omitempty" yaml:"udp_relay,omitempty"`
}

type TransportOptions struct {
	Type                     string                 `json:"type,omitempty" yaml:"type,omitempty"`
	HeaderType               string                 `json:"header_type,omitempty" yaml:"header_type,omitempty"`
	Method                   string                 `json:"method,omitempty" yaml:"method,omitempty"`
	Path                     string                 `json:"path,omitempty" yaml:"path,omitempty"`
	Host                     string                 `json:"host,omitempty" yaml:"host,omitempty"`
	Hosts                    []string               `json:"hosts,omitempty" yaml:"hosts,omitempty"`
	Headers                  map[string]string      `json:"headers,omitempty" yaml:"headers,omitempty"`
	ServiceName              string                 `json:"service_name,omitempty" yaml:"service_name,omitempty"`
	MaxEarlyData             int                    `json:"max_early_data,omitempty" yaml:"max_early_data,omitempty"`
	EarlyDataHeaderName      string                 `json:"early_data_header_name,omitempty" yaml:"early_data_header_name,omitempty"`
	V2RayHTTPUpgrade         bool                   `json:"v2ray_http_upgrade,omitempty" yaml:"v2ray_http_upgrade,omitempty"`
	V2RayHTTPUpgradeFastOpen bool                   `json:"v2ray_http_upgrade_fast_open,omitempty" yaml:"v2ray_http_upgrade_fast_open,omitempty"`
	XHTTP                    *XHTTPTransportOptions `json:"xhttp,omitempty" yaml:"xhttp,omitempty"`
}

type XHTTPTransportOptions struct {
	Mode             string                 `json:"mode,omitempty" yaml:"mode,omitempty"`
	ReuseSettings    *XHTTPReuseSettings    `json:"reuse_settings,omitempty" yaml:"reuse_settings,omitempty"`
	DownloadSettings *XHTTPDownloadSettings `json:"download_settings,omitempty" yaml:"download_settings,omitempty"`
}

type XHTTPReuseSettings struct {
	MaxConcurrency   string `json:"max_concurrency,omitempty" yaml:"max_concurrency,omitempty"`
	MaxConnections   string `json:"max_connections,omitempty" yaml:"max_connections,omitempty"`
	CMaxReuseTimes   string `json:"c_max_reuse_times,omitempty" yaml:"c_max_reuse_times,omitempty"`
	HMaxRequestTimes string `json:"h_max_request_times,omitempty" yaml:"h_max_request_times,omitempty"`
	HMaxReusableSecs string `json:"h_max_reusable_secs,omitempty" yaml:"h_max_reusable_secs,omitempty"`
	HKeepAlivePeriod int    `json:"h_keep_alive_period,omitempty" yaml:"h_keep_alive_period,omitempty"`
}

type XHTTPDownloadSettings struct {
	Server        *string             `json:"server,omitempty" yaml:"server,omitempty"`
	Port          *uint16             `json:"port,omitempty" yaml:"port,omitempty"`
	Path          *string             `json:"path,omitempty" yaml:"path,omitempty"`
	Host          *string             `json:"host,omitempty" yaml:"host,omitempty"`
	Headers       *map[string]string  `json:"headers,omitempty" yaml:"headers,omitempty"`
	TLS           *TLSOptions         `json:"tls,omitempty" yaml:"tls,omitempty"`
	ReuseSettings *XHTTPReuseSettings `json:"reuse_settings,omitempty" yaml:"reuse_settings,omitempty"`
}

type MultiplexOptions struct {
	Enabled        bool   `json:"enabled" yaml:"enabled"`
	Protocol       string `json:"protocol,omitempty" yaml:"protocol,omitempty"`
	MaxConnections int    `json:"max_connections,omitempty" yaml:"max_connections,omitempty"`
	MinStreams     int    `json:"min_streams,omitempty" yaml:"min_streams,omitempty"`
	MaxStreams     int    `json:"max_streams,omitempty" yaml:"max_streams,omitempty"`
	Padding        bool   `json:"padding,omitempty" yaml:"padding,omitempty"`
}

type ECHOptions struct {
	Enabled         bool     `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	Config          []string `json:"config,omitempty" yaml:"config,omitempty"`
	QueryServerName string   `json:"query_server_name,omitempty" yaml:"query_server_name,omitempty"`
	DNS             string   `json:"dns,omitempty" yaml:"dns,omitempty"`
	ForceQuery      string   `json:"force_query,omitempty" yaml:"force_query,omitempty"`
}

type RealityOptions struct {
	Enabled   bool   `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	PublicKey string `json:"public_key,omitempty" yaml:"public_key,omitempty"`
	ShortID   string `json:"short_id,omitempty" yaml:"short_id,omitempty"`
}

type UDPOverTCPOptions struct {
	Enabled bool `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	Version int  `json:"version,omitempty" yaml:"version,omitempty"`
}

type ShadowsocksROptions struct {
	Protocol      string `json:"protocol,omitempty" yaml:"protocol,omitempty"`
	ProtocolParam string `json:"protocol_param,omitempty" yaml:"protocol_param,omitempty"`
	Obfs          string `json:"obfs,omitempty" yaml:"obfs,omitempty"`
	ObfsParam     string `json:"obfs_param,omitempty" yaml:"obfs_param,omitempty"`
}

type SnellOptions struct {
	Version           int               `json:"version,omitempty" yaml:"version,omitempty"`
	Obfs              string            `json:"obfs,omitempty" yaml:"obfs,omitempty"`
	ObfsHost          string            `json:"obfs_host,omitempty" yaml:"obfs_host,omitempty"`
	Reuse             *bool             `json:"reuse,omitempty" yaml:"reuse,omitempty"`
	ClientFingerprint string            `json:"client_fingerprint,omitempty" yaml:"client_fingerprint,omitempty"`
	ShadowTLS         *ShadowTLSOptions `json:"shadow_tls,omitempty" yaml:"shadow_tls,omitempty"`
}

type ShadowTLSOptions struct {
	Password           string   `json:"password,omitempty" yaml:"password,omitempty"`
	Host               string   `json:"host,omitempty" yaml:"host,omitempty"`
	Version            int      `json:"version,omitempty" yaml:"version,omitempty"`
	ALPN               []string `json:"alpn,omitempty" yaml:"alpn,omitempty"`
	Fingerprint        string   `json:"fingerprint,omitempty" yaml:"fingerprint,omitempty"`
	Certificate        string   `json:"certificate,omitempty" yaml:"certificate,omitempty"`
	PrivateKey         string   `json:"private_key,omitempty" yaml:"private_key,omitempty"`
	InsecureSkipVerify bool     `json:"insecure_skip_verify,omitempty" yaml:"insecure_skip_verify,omitempty"`
}

type AnyTLSOptions struct {
	IdleSessionCheckInterval string `json:"idle_session_check_interval,omitempty" yaml:"idle_session_check_interval,omitempty"`
	IdleSessionTimeout       string `json:"idle_session_timeout,omitempty" yaml:"idle_session_timeout,omitempty"`
	MinIdleSession           int    `json:"min_idle_session,omitempty" yaml:"min_idle_session,omitempty"`
}

type HysteriaOptions struct {
	Protocol     string                `json:"protocol,omitempty" yaml:"protocol,omitempty"`
	ServerPorts  []string              `json:"server_ports,omitempty" yaml:"server_ports,omitempty"`
	HopInterval  string                `json:"hop_interval,omitempty" yaml:"hop_interval,omitempty"`
	Up           string                `json:"up,omitempty" yaml:"up,omitempty"`
	Down         string                `json:"down,omitempty" yaml:"down,omitempty"`
	UpMbps       int                   `json:"up_mbps,omitempty" yaml:"up_mbps,omitempty"`
	DownMbps     int                   `json:"down_mbps,omitempty" yaml:"down_mbps,omitempty"`
	Auth         string                `json:"auth,omitempty" yaml:"auth,omitempty"`
	AuthString   string                `json:"auth_str,omitempty" yaml:"auth_str,omitempty"`
	Obfs         string                `json:"obfs,omitempty" yaml:"obfs,omitempty"`
	ObfsPassword string                `json:"obfs_password,omitempty" yaml:"obfs_password,omitempty"`
	Realm        *HysteriaRealmOptions `json:"realm,omitempty" yaml:"realm,omitempty"`
	BBRProfile   string                `json:"bbr_profile,omitempty" yaml:"bbr_profile,omitempty"`
	CWND         int                   `json:"cwnd,omitempty" yaml:"cwnd,omitempty"`
	UDPMTU       int                   `json:"udp_mtu,omitempty" yaml:"udp_mtu,omitempty"`
	QUIC         map[string]any        `json:"quic,omitempty" yaml:"quic,omitempty"`
}

type HysteriaRealmOptions struct {
	Enabled     bool     `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	ServerURL   string   `json:"server_url,omitempty" yaml:"server_url,omitempty"`
	Token       string   `json:"token,omitempty" yaml:"token,omitempty"`
	RealmID     string   `json:"realm_id,omitempty" yaml:"realm_id,omitempty"`
	STUNServers []string `json:"stun_servers,omitempty" yaml:"stun_servers,omitempty"`
}

type TUICOptions struct {
	CongestionControl    string `json:"congestion_control,omitempty" yaml:"congestion_control,omitempty"`
	UDPRelayMode         string `json:"udp_relay_mode,omitempty" yaml:"udp_relay_mode,omitempty"`
	ZeroRTTHandshake     bool   `json:"zero_rtt_handshake,omitempty" yaml:"zero_rtt_handshake,omitempty"`
	ReduceRTT            bool   `json:"reduce_rtt,omitempty" yaml:"reduce_rtt,omitempty"`
	Heartbeat            string `json:"heartbeat,omitempty" yaml:"heartbeat,omitempty"`
	UDPOverStream        bool   `json:"udp_over_stream,omitempty" yaml:"udp_over_stream,omitempty"`
	UDPOverStreamVersion int    `json:"udp_over_stream_version,omitempty" yaml:"udp_over_stream_version,omitempty"`
}

type MieruOptions struct {
	PortRange      string `json:"port_range,omitempty" yaml:"port_range,omitempty"`
	Transport      string `json:"transport,omitempty" yaml:"transport,omitempty"`
	Multiplexing   string `json:"multiplexing,omitempty" yaml:"multiplexing,omitempty"`
	HandshakeMode  string `json:"handshake_mode,omitempty" yaml:"handshake_mode,omitempty"`
	TrafficPattern string `json:"traffic_pattern,omitempty" yaml:"traffic_pattern,omitempty"`
}

type WireGuardOptions struct {
	PrivateKey          string          `json:"private_key,omitempty" yaml:"private_key,omitempty"`
	Address             []string        `json:"address,omitempty" yaml:"address,omitempty"`
	IP                  string          `json:"ip,omitempty" yaml:"ip,omitempty"`
	IPv6                string          `json:"ipv6,omitempty" yaml:"ipv6,omitempty"`
	Peers               []WireGuardPeer `json:"peers,omitempty" yaml:"peers,omitempty"`
	MTU                 int             `json:"mtu,omitempty" yaml:"mtu,omitempty"`
	Workers             int             `json:"workers,omitempty" yaml:"workers,omitempty"`
	PersistentKeepalive int             `json:"persistent_keepalive,omitempty" yaml:"persistent_keepalive,omitempty"`
	Reserved            []uint8         `json:"reserved,omitempty" yaml:"reserved,omitempty"`
}

type WireGuardPeer struct {
	Server              string   `json:"server,omitempty" yaml:"server,omitempty"`
	Port                uint16   `json:"port,omitempty" yaml:"port,omitempty"`
	PublicKey           string   `json:"public_key,omitempty" yaml:"public_key,omitempty"`
	PreSharedKey        string   `json:"pre_shared_key,omitempty" yaml:"pre_shared_key,omitempty"`
	AllowedIPs          []string `json:"allowed_ips,omitempty" yaml:"allowed_ips,omitempty"`
	Reserved            []uint8  `json:"reserved,omitempty" yaml:"reserved,omitempty"`
	PersistentKeepalive int      `json:"persistent_keepalive,omitempty" yaml:"persistent_keepalive,omitempty"`
}
