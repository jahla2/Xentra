# Timestamped Evidence and Incident Timeline

Every investigation evidence item carries:

- source
- success
- output
- occurredAt
- durationMs

The control plane records evidence collection time in UTC and duration in milliseconds. Evidence created by validation or error paths is normalized before it reaches the AI or incident store.

When an incident is created, infrastructure evidence is normalized into timeline events and merged with GitHub commit/workflow events. The resulting timeline is sorted chronologically.

This gives one incident chronology across infrastructure observations and repository activity instead of treating evidence and source-control events as unrelated lists.
