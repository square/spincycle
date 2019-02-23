---
layout: default
title: Networking
parent: Learn More
nav_order: 3
---

# Networking

![Spin Cycle Networking](/spincycle/assets/img/spincycle_networking.svg)

The top diagram shows the simplest case: a single Request Manager (RM) API communicates with a single Job Runner (JR) API. Users, through the spinc CLI, only communicate with the RM, which communicates with the JR on their behalf.

Dotted lines indicate non-specific connections. Generally, a user (via spinc) does not use a specific Request Manager instance. When a request is started, the RM sends it to any JR to run it.

Solid lines indicate specific connections. When a user (via spinc) wants the status of a request, the RM connects directly to the JR running the request.

The bottom diagram is a typical production deployment: N-many RM and N-many JR, both behind load balancers. Users communicate with any RM, and the RM run requests on any JR via load balancing. The RM still communicate directly with specific JR to get request status (solid line).

JR instances report [server.addr](/spincycle/v1.0/operate/configure.html#jr.server.addr) as their address.
