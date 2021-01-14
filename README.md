# Akari

## 1. Introduction

This project is:

- A TLS termination proxy
- Routed traffic via TLS SNI (Server Name Indication)
- Implement TCP proxy, HTTP proxy and SOCKS5 proxy over TLS
- Implement multiplexing on server and agent via smux
- Implement connection pool on agent

This project is inspired by ESNI and wildcard certs:

- When server name is encrypted in TLS handshake, it can be used as a key to route traffic
-  When wildcard certs is deployed on server, we do not need to add DNS for every sub domain, SNI can be set on agent

## 2. Requirement

- Domain
- Wildcard Cert
- Virtual Private Server

### 2.1 Domain

Cloudflare, DNSPod, Aliyun or any other provider that supports DNS API to issue wildcard cert.

### 2.2 Wildcard Cert

Refer to [acme.sh](https://github.com/acmesh-official/acme.sh)

### 2.3 Virtual Private Server

- [BandwagonHost](https://bwh88.net/aff.php?aff=32903)
- [Vultr](https://www.vultr.com/?ref=6873289)
- ...

VPS with AES and AVX2 instruction set is recommended.

## 3. Configuration

### 3.1 Server

Server listens on one public address and route traffic according to SNI proxy config.

**akari config**

|Field|Type|Comment|
|:---|:---|:---|
|LogLevel|int|debug=5, info=4, warn=3, error=2, fatal=1, panic=0 (default 4)|
|Mode|string|**server** or **agent**, use **server** on server|
|Addr|string|listening address of server|
|Conf|string|SNI based proxy config folder path, all json files under this folder are loaded on start|
|HTTPRedirect|bool|enable http redirect for https mode sni|
|TLS|object|TLS config, contains ForwardSecurity switchy and a group of TLS certs|

**SNI based proxy config**

|Field|Type|Comment|
|:---|:---|:---|
|sni|string|server name|
|mode|string|tcp, socks5 and https are supported|
|auth|string|**user:password** format auth string, supported by socks5 and https mode|
|mux|bool|multiplexing conn switch|
|addr|string|dst addr, supported by tcp mode|
|ReverseProxy|map[string]string|http path and dst addr, supported by https mode|

mode in this config:

- tcp: tcp proxy, offloading TLS and redirect tcp flow to dst addr
- socks5: socks5 proxy over tls, support auth and no auth, only connect is implemented currently
- https: https proxy,  support auth and no auth,  only connect is implemented

### 3.2 Agent

Agent works as a TLS forward proxy at local, listens multiple address according to SNI proxy config and redirect traffic to corresponding server.

**akari config**

|Field|Type|Comment|
|:---|:---|:---|
|LogLevel|int|debug=5, info=4, warn=3, error=2, fatal=1, panic=0 (default 4)|
|Mode|string|**server** or **agent**, use **agent** on agent|
|Conf|string|SNI based proxy config folder path, all 

**SNI based proxy config**

|Field|Type|Comment|
|:---|:---|:---|
|sni|string|server name|
|remote|string|remote server address|
|local|string|local listeing address|
|mux|bool|multiplexing conn switch|
|pool|bool|conn pool switch|
|maxIdle|int|max idle mux conn when conn pool is enabled|
|maxMux|int|max multiplexing conn on one underlying mux conn when conn pool is enabled|

## 4. Example

### 4.1 Server

**/etc/akari**

```
➜  ~ tree /etc/akari
/etc/akari
├── akari.json
├── cert
│   ├── example_ecc.key
│   ├── example_ecc.pem
│   ├── example.key
│   └── example.pem
└── conf
    ├── mux-tcp.json
    ├── mux-https.json
    ├── mux-socks5.json
    ├── tcp.json
    ├── https.json
    └── socks5.json
```

**/etc/akari/akari.json**

```
{
    "LogLevel": 5,
    "Mode": "server",
    "Addr": "0.0.0.0:443",
    "Conf": "/etc/akari/conf",
    "TLS": {
        "ForwardSecurity": true,
        "Certs": [
            {
                "Cert": "/etc/akari/cert/example_ecc.pem",
                "Key": "/etc/akari/cert/example_ecc.key"
            },
            {
                "Cert": "/etc/akari/cert/example.pem",
                "Key": "/etc/akari/cert/example.key"
            }
        ]
    }
}
```

**socks5 proxy and multiplexing socks5 proxy**

/etc/akari/conf/socks5.json

```
{
    "sni":"xxxxx-socks5.example.com",
    "auth":"user:password",
    "mode":"socks5"
}
```
/etc/akari/conf/mux-socks5.json

```
{
    "sni":"xxxxx-mux-socks5.example.com",
    "mode":"socks5",
    "auth":"user:password",
    "mux": true
}
```

**https proxy and multiplexing https proxy**

/etc/akari/conf/https.json

```
{
    "sni":"xxxxx-https.example.com",
    "auth":"user:password",
    "mode":"https",
    "reverseProxy": {
        "/23336666":"127.0.0.1:23336",
        "/66662333":"127.0.0.1:62333"
    }
}
```

/etc/akari/conf/mux-https.json

```
{
    "sni":"xxxxx-mux-https.example.com",
    "mode":"https",
    "auth":"user:password",
    "mux": true
}
```

**tcp proxy and multiplexing tcp proxy**

/etc/akari/conf/tcp.json
```
{
    "sni":"xxxxx-tcp.example.com",
    "mode":"tcp",
    "addr": "127.0.0.1:8080"
}
```
/etc/akari/conf/mux-tcp.json
```
{
    "sni":"xxxxx-tcp.example.com",
    "mode":"tcp",
    "addr": "127.0.0.1:8080",
    "mux": true
}
```
### 4.2 Agent

**/etc/akari**

```
/etc/akari
├── akari.json
└── conf
    ├── mux-tcp.json
    ├── mux-https.json
    ├── mux-socks5.json
    ├── tcp.json
    ├── https.json
    └── socks5.json
```

**/etc/akari/akari.json**

```
{
    "LogLevel": 5,
    "Mode": "agent",
    "Conf": "/etc/akari/conf"
}
```

**socks5 proxy and multiplexing socks5 proxy**

/etc/akari/conf/socks5.json

```
{
        "sni": "xxxxx-socks5.example.com",
        "remote": "xx.xx.xx.xx:443",
        "local": "0.0.0.0:1080"
}
```
/etc/akari/conf/mux-socks5.json

```
{
        "sni": "xxxxx-mux-socks5.example.com",
        "remote": "xx.xx.xx.xx:443",
        "local": "0.0.0.0:2080",
        "mux": true,
        "pool": true,
        "maxIdle": 8,
        "maxMux": 8
}
```

Since agent only works as a TLS forward proxy at local, we don't store application layer auth here, tcp and https proxy format are the same just the same expect for sni, remote and local field.

### 4.3 Start  Service

Run command: **/usr/local/bin/akari -c /etc/akari/akari.json**