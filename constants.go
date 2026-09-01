package sensorswave

import "strconv"

// HTTP header keys used on outbound SDK requests.
const (
	HeaderProject         = "Project"
	HeaderSourceToken     = "SourceToken"
	HeaderAccountAPIToken = "AccountApiToken"
)

// Predefined event names emitted by the SDK.
const (
	PseIdentify       = "$Identify"       // User correlation event
	PseFeatureImpress = "$FeatureImpress" // Feature impression event (Gate/Config)
	PseExpImpress     = "$ExpImpress"     // Experiment impression event
	PseUserSet        = "$UserSet"        // User property event (umbrella for all profile_* operations)
)

// Predefined property keys emitted by the SDK.
//
// `Psp*Lib*` are runtime variables (set by SDK at normalize time);
// the rest are SDK-injected user/device context.
const (
	PspFeatureKey     = "$feature_key"
	PspFeatureVariant = "$feature_variant"
	PspExpKey         = "$exp_key"
	PspExpVariant     = "$exp_variant"

	PspLib        = "$lib"             // v:string -- SDK library name
	PspLibVersion = "$lib_version"     // v:string -- SDK library version
	PspAppVer     = "$app_version"     // v:string -- app version
	PspBrowser    = "$browser"         // v:string -- browser name
	PspBrowserVer = "$browser_version" // v:string -- browser version
	PspModel      = "$model"           // v:string -- device model
	PspIP         = "$ip"              // v:string -- IP address
	PspOS         = "$os"              // v:string -- operating system: ios/android/harmony
	PspOSVer      = "$os_version"      // v:string -- OS version
	PspCountry    = "$country"         // v:string -- country (set by SDK or GeoIP)
	PspProvince   = "$province"        // v:string -- province/state (set by SDK or GeoIP)
	PspCity       = "$city"            // v:string -- city (set by SDK or GeoIP)
)

// FormatFeaturePropertyName returns the feature user property name in the format "$feature_{ID}".
func FormatFeaturePropertyName(id int) string {
	return "$feature_" + strconv.Itoa(id)
}

// FormatExpPropertyName returns the experiment user property name in the format "$exp_{ID}".
func FormatExpPropertyName(id int) string {
	return "$exp_" + strconv.Itoa(id)
}
