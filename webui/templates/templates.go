package templates

import (
	_ "embed"
	html "html/template"
)

// embeds the header.html template
//
//go:embed header.html
var BytesHeaderTemplate []byte
var HeaderTemplate, _ = html.New("header").Parse(string(BytesHeaderTemplate))

// embeds the i2ptunnelconfig.html template
//
//go:embed i2ptunnelconfig.html
var BytesI2PTunnelConfigTemplate []byte
var I2PTunnelConfigTemplate, _ = html.New("i2ptunnelconfig").Parse(string(BytesI2PTunnelConfigTemplate))

// embeds the i2ptunnelcontrol.html template
//
//go:embed i2ptunnelcontrol.html
var BytesI2PTunnelControlTemplate []byte
var I2PTunnelControlTemplate, _ = html.New("i2ptunnelcontrol").Parse(string(BytesI2PTunnelControlTemplate))

// embeds the i2ptunnelminicontrol.html template
//
//go:embed i2ptunnelminicontrol.html
var BytesI2PTunnelMiniControlTemplate []byte
var I2PTunnelMiniControlTemplate, _ = html.New("i2ptunnelminicontrol").Parse(string(BytesI2PTunnelMiniControlTemplate))

// embeds the i2ptunnelgroup.html template
//
//go:embed i2ptunnelgroup.html
var BytesI2PTunnelGroupTemplate []byte
var I2PTunnelGroupTemplate, _ = html.New("i2ptunnelgroup").Parse(string(BytesI2PTunnelGroupTemplate))

// embeds the footer.html template
//
//go:embed footer.html
var BytesFooterTemplate []byte
var FooterTemplate, _ = html.New("footer").Parse(string(BytesFooterTemplate))
