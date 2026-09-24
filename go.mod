module keylint

go 1.27

require (
	github.com/anthropics/anthropic-sdk-go v1.75.0
	github.com/google/wire v0.7.0
	github.com/joho/godotenv v1.5.1
	github.com/minio/selfupdate v0.6.0
	github.com/openai/openai-go/v3 v3.66.0
	github.com/wailsapp/wails/v3 v3.0.0-beta.25
	github.com/zalando/go-keyring v0.2.8
)

require (
	aead.dev/minisign v0.2.0 // indirect
	github.com/adrg/xdg v0.5.3 // indirect
	github.com/bahlo/generic-list-go v0.2.0 // indirect
	github.com/buger/jsonparser v1.1.2 // indirect
	github.com/coder/websocket v1.8.15 // indirect
	github.com/danieljoos/wincred v1.2.3 // indirect
	github.com/go-ole/go-ole v1.3.0 // indirect
	github.com/godbus/dbus/v5 v5.2.2 // indirect
	github.com/invopop/jsonschema v0.14.0 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/pb33f/ordered-map/v2 v2.3.1 // indirect
	github.com/standard-webhooks/standard-webhooks/libraries v0.0.1 // indirect
	github.com/tidwall/gjson v1.19.0 // indirect
	github.com/tidwall/match v1.1.1 // indirect
	github.com/tidwall/pretty v1.2.1 // indirect
	github.com/tidwall/sjson v1.2.5 // indirect
	go.yaml.in/yaml/v4 v4.0.0-rc.2 // indirect
	golang.org/x/crypto v0.55.0 // indirect
	golang.org/x/sync v0.22.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
)

// Go 1.27 lets SSL_CERT_FILE / SSL_CERT_DIR replace the Windows and macOS
// certificate store. Keep the pre-1.27 behaviour: KeyLint trusts the system
// store there, so a stale variable cannot break every provider call and a
// corporate root CA installed only in the store keeps working. Revisit as an
// explicit product decision, not as a side effect of a toolchain bump.
godebug x509sslcertoverrideplatform=0
