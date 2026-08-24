package client

import "time"

const (
	// AuthTokenTimelife - Time duration after which we assume auth token expired
	AuthTokenTimelife = 60 * time.Minute

	// AuthTokenRenewMargin - how long before its assumed expiry a token is
	// treated as stale. The websocket binds the token at dial time and cannot
	// renegotiate it, so a token that would expire mid-connection is renewed
	// before the connection is opened rather than after it silently dies.
	AuthTokenRenewMargin = 5 * time.Minute

	// keepaliveInterval - how often a keepalive frame is pushed to the camera
	keepaliveInterval = 20 * time.Second

	// livenessProbeAfter - how long the connection may stay silent before we
	// stop trusting it and ask the camera a question we know it answers.
	// Keepalives are one-way, so silence alone never proves the link is up.
	livenessProbeAfter = 60 * time.Second

	// livenessProbeTimeout - how long the camera has to answer a liveness
	// probe before the connection is treated as dead and torn down
	livenessProbeTimeout = 30 * time.Second

	// writeTimeout - upper bound on a single websocket write, so a stalled
	// connection surfaces as an error instead of blocking the sender forever
	writeTimeout = 10 * time.Second

	// minConnectionLifetime - floor on how long a connection is kept before it
	// is retired for token renewal, so an unexpectedly old token cannot turn
	// the renewal into a reconnect loop
	minConnectionLifetime = 1 * time.Minute

	// connectionRenewGrace - how far past the point at which MaybeAuthorize
	// starts treating a token as stale a connection is retired. Retiring it
	// exactly on that boundary is a coin flip on whether the reconnect renews
	// the token or reuses the old one and has to come straight back.
	connectionRenewGrace = 30 * time.Second
)
