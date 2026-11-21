# Security policy

## Reporting a vulnerability

Please do not open a public issue. Email **opensource@lakshyapocketlab.com** with a description,
reproduction steps and the affected version. We aim to acknowledge reports within 3 business days
and to ship a fix or mitigation for confirmed issues promptly.

## Scope

This SDK sends your `hello_live_...` API key as a Bearer token. Never commit keys; load them from
the environment (`HELLO_API_KEY`). The SDK never logs request headers or bodies.

## Supported versions

Only the latest minor release receives security fixes.
