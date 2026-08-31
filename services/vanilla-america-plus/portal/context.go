package main

import "context"

type sessionContextKey struct{}

func withSession(ctx context.Context, session Session) context.Context {
	return context.WithValue(ctx, sessionContextKey{}, session)
}

func sessionFromContext(ctx context.Context) Session {
	session, _ := ctx.Value(sessionContextKey{}).(Session)
	return session
}
