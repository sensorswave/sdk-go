package sensorswave

// Predefined constants for events and properties
import (
	"strconv"
)

// Predefined events
const (
	PseIdentify       = "$Identify"       // User correlation event
	PseFeatureImpress = "$FeatureImpress" // Feature impression event (Gate/Config)
	PseExpImpress     = "$ExpImpress"     // Experiment impression event
	// Internal events from def package
	PseUserSet = "$UserSet" // User property event
)

// User property operation types
const (
	UserSetTypeSet       = "user_set"       // Set user property
	UserSetTypeSetOnce   = "user_set_once"  // Set once user property
	UserSetTypeIncrement = "user_increment" // Increment user property
	UserSetTypeAppend    = "user_append"    // Append user property
	UserSetTypeUnion     = "user_union"     // Union user property
	UserSetTypeUnset     = "user_unset"     // Unset user property
	UserSetTypeDelete    = "user_delete"    // Delete user property
)

// Predefined property keys
const (
	PspUserSetType    = "$user_set_type" // User property set type
	PspFeatureKey     = "$feature_key"
	PspFeatureVariant = "$feature_variant"
	PspExpKey         = "$exp_key"
	PspExpVariant     = "$exp_variant"
)

// Predefined properties
const (
	PspLib        = "$lib"             // v:string      -- SDK library name
	PspLibVersion = "$lib_version"     // v:string      -- SDK library version
	PspAppVer     = "$app_version"     // v:string      -- app version
	PspBrowser    = "$browser"         // v:string      -- browser name
	PspBrowserVer = "$browser_version" // v:string      -- browser version
	PspModel      = "$model"           // v:string      -- device model
	PspIP         = "$ip"              // v:string      -- IP address
	PspOS         = "$os"              // v:string      -- operating system: ios/android/harmony
	PspOSVer      = "$os_version"      // v:string      -- OS version
	PspCountry    = "$country"         // v:string      -- country (set by SDK or GeoIP)
	PspProvince   = "$province"        // v:string      -- province/state (set by SDK or GeoIP)
	PspCity       = "$city"            // v:string      -- city (set by SDK or GeoIP)
)

// FormatFeaturePropertyName returns the feature user property name in the format "$feature_{ID}".
func FormatFeaturePropertyName(id int) string {
	return "$feature_" + strconv.Itoa(id)
}

// FormatExpPropertyName returns the experiment user property name in the format "$exp_{ID}".
func FormatExpPropertyName(id int) string {
	return "$exp_" + strconv.Itoa(id)
}
