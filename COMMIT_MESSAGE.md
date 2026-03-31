# Commit Message

## Summary (50 chars max)
```
feat: Add log filtering parameters to API
```

## Body (72 chars per line)
```
Add Kubernetes-style log filtering parameters (tail, since, since_time)
to the application logs API endpoints to address overwhelming log volume.

Parameters:
- tail: Limit to last N lines from the end
- since: Show logs from duration ago (e.g., "1h", "30m")
- since_time: Show logs since RFC3339 timestamp
- follow: Stream logs in real-time

Implementation:
- Added LogParameters struct with Tail, Since, SinceTime, and Follow
- Parse and validate query parameters in API layer
- Apply parameters to tailer.Config in application layer
- Only set SinceSeconds in K8s API if > 0 to avoid API issues
- Added comprehensive debug logging at each layer

API Usage:
  GET /api/v1/namespaces/:ns/applications/:app/logs?tail=100&since=1h
  GET /api/v1/namespaces/:ns/staging/:id/logs?tail=50&follow=true

Changes:
- internal/api/v1/application/logs.go: Parse params, set in LogParameters
- internal/application/application.go: Apply params to tailer config
- helpers/kubernetes/tailer/tailer.go: Fix SinceSeconds handling
- pkg/api/core/v1/client/apps.go: Add LogOptions for client support

This significantly reduces log volume by allowing users to limit the
number of lines and filter by time ranges, making logs more manageable
and less overwhelming.
```

---

## Alternative: Shorter Version

### Summary
```
feat: Add log filtering (tail, since, since_time) to API
```

### Body
```
Add query parameters to filter application logs and reduce volume:

- tail: Last N lines
- since: Duration (e.g., "1h")
- since_time: RFC3339 timestamp
- follow: Stream in real-time

Parameters are validated in API layer, applied to tailer config in
application layer, and passed to Kubernetes API. Fixed SinceSeconds
handling to only set when > 0.

Usage: GET /logs?tail=100&since=1h&follow=true

Addresses issue of overwhelming log dumps by allowing users to limit
and filter log output.
```

---

## Alternative: Detailed Version

### Summary
```
feat(api): Add pagination/filtering for application logs
```

### Body
```
Add Kubernetes-style log filtering to reduce overwhelming log volume.

Problem:
Users reported that logs are dumped without any pagination or filtering,
making them overwhelming and difficult to navigate.

Solution:
Implement query parameters for log filtering:
- tail: Number of lines from the end (integer)
- since: Duration string (e.g., "1h", "30m", "1s")  
- since_time: RFC3339 timestamp for absolute time filtering
- follow: Boolean to enable log streaming

Implementation Details:
1. API Layer (internal/api/v1/application/logs.go)
   - Extract and validate query parameters
   - Create LogParameters struct with all fields including Follow
   - Log both parsed and raw parameter values for debugging

2. Application Layer (internal/application/application.go)
   - Add LogParameters struct: Tail, Since, SinceTime, Follow
   - Remove separate follow parameter from Logs() signature
   - Apply parameters to tailer.Config
   - Extract follow from logParams.Follow

3. Tailer Layer (helpers/kubernetes/tailer/tailer.go)
   - Only set SinceSeconds if > 0 (fixes Kubernetes API edge case)
   - Add debug logging for Kubernetes API calls

4. Client Layer (pkg/api/core/v1/client/apps.go)
   - Already has LogOptions support for client usage
   - Properly formats parameters for query string

API Examples:
  # Last 100 lines
  GET /logs?tail=100
  
  # Logs from last hour
  GET /logs?since=1h
  
  # Since specific time
  GET /logs?since_time=2023-01-01T00:00:00Z
  
  # Combined with follow
  GET /logs?tail=50&since=30m&follow=true

Benefits:
- Significantly reduces log volume for users
- Native Kubernetes-style parameters (familiar interface)
- Backward compatible (all parameters optional)
- Proper validation with error messages
- Debug logging at each layer for troubleshooting

Fixes overwhelming log dumps by giving users control over log volume.
```

---

## Recommended Choice: **Shorter Version**

The shorter version is clear, concise, and hits all the important points without being too verbose. It's perfect for a commit message that will be reviewed in a PR.

Copy the "Shorter Version" for your commit!
