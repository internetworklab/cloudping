package cli

// Note: not all CLIs support all methods,
// it's in their own discretion (up to the CLI's implementation)
// to decide what auth methods to support.
type AuthenticationMethod string

const AuthenticationMethodCloudflare AuthenticationMethod = "cloudflare"
const AuthenticationMethodJWT AuthenticationMethod = "jwt"
const AuthenticationMethodNone AuthenticationMethod = "none"
