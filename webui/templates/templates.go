// Package templates provides embedded HTML templates for the web UI.
package templates

import (
	_ "embed"
	html "html/template"
)

// embeds the header.html template
//
//go:embed header.html
var BytesHeaderTemplate []byte

// HeaderTemplate is parsed at init time; template.Must panics immediately
// with a clear error if the embedded HTML contains a syntax error.
var HeaderTemplate = html.Must(html.New("header").Parse(string(BytesHeaderTemplate)))

// embeds the i2ptunnelconfig.html template
//
//go:embed i2ptunnelconfig.html
var BytesI2PTunnelConfigTemplate []byte
var I2PTunnelConfigTemplate = html.Must(html.New("i2ptunnelconfig").Parse(string(BytesI2PTunnelConfigTemplate)))

// embeds the i2ptunnelcontrol.html template
//
//go:embed i2ptunnelcontrol.html
var BytesI2PTunnelControlTemplate []byte
var I2PTunnelControlTemplate = html.Must(html.New("i2ptunnelcontrol").Parse(string(BytesI2PTunnelControlTemplate)))

// embeds the i2ptunnelminicontrol.html template
//
//go:embed i2ptunnelminicontrol.html
var BytesI2PTunnelMiniControlTemplate []byte
var I2PTunnelMiniControlTemplate = html.Must(html.New("i2ptunnelminicontrol").Parse(string(BytesI2PTunnelMiniControlTemplate)))

// embeds the i2ptunnelgroup.html template
//
//go:embed i2ptunnelgroup.html
var BytesI2PTunnelGroupTemplate []byte
var I2PTunnelGroupTemplate = html.Must(html.New("i2ptunnelgroup").Parse(string(BytesI2PTunnelGroupTemplate)))

// embeds the footer.html template
//
//go:embed footer.html
var BytesFooterTemplate []byte
var FooterTemplate = html.Must(html.New("footer").Parse(string(BytesFooterTemplate)))
