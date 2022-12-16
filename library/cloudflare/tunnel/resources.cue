// The Ingress resource class configures a CF tunnel to route external traffic
// to the local cluster ingress. This removes the need to expose the ingress'
// address to the public internet. It also removes the need to have a public
// address, which may save you a few bucks in cloud resources.
//
// To use, have a server declare an Ingress resource, using a previously
// configured tunnel. You can create a new tunnel using:
//
//  $ cloudflared tunnel create
//
// And then store the resulting cert.pem and tunnel credentials as secrets (you
// can then remove these files from your local filesystem).
//
// Attach the following to your server:
//
//  resources: {
//	  ingress: {
//      class: "namespacelabs.dev/foundation/library/cloudflare/tunnel:Ingress"
//      intent: hostname: ["xyz.foobar.com", "foobar.com"]
//    }
//  }
//
// And deploy. You're set.

resourceClasses: {
	"Ingress": {
		description: "Cloudflare Tunnel Ingress"
		intent: {
			type:   "library.cloudflare.tunnel.IngressIntent"
			source: "./types.proto"
		}
		produces: {
			type:   "library.cloudflare.tunnel.IngressInstance"
			source: "./types.proto"
		}
		defaultProvider: "namespacelabs.dev/foundation/library/cloudflare/tunnel"
	}
}

providers: {
	"namespacelabs.dev/foundation/library/cloudflare/tunnel:Ingress": {
		initializedWith: {
			imageFrom: binary: "namespacelabs.dev/foundation/library/cloudflare/tunnel/prepare"

			env: {
				CF_TUNNEL_CREDENTIALS: fromSecret: "namespacelabs.dev/foundation/library/cloudflare/tunnel/server:cfTunnelCredentials"
				CF_TUNNEL_CERT_PEM: fromSecret:    ":cfTunnelCertPem"
			}
		}

		resources: {
			server: {
				class:  "namespacelabs.dev/foundation/library/runtime:Server"
				intent: "namespacelabs.dev/foundation/library/cloudflare/tunnel/server"
			}
		}
	}
}

secrets: {
	cfTunnelCertPem: {
		description: "Cloudflare Tunnel cert.pem"
	}
}
