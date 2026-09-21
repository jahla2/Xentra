# GitHub Correlation and Incidents

A GitHub repository can be linked to one Xentra environment. The access token is encrypted through the same credential vault used by SSH and is never included in AI context.

During incident creation Xentra:

1. collects environment evidence;
2. asks the investigation service for probable root cause;
3. fetches recent GitHub commits and workflow runs;
4. normalizes GitHub activity into timeline events;
5. stores the incident, evidence and timeline.

The current MVP uses a GitHub access token. The client is compatible with short-lived GitHub App installation tokens, which is the recommended production credential source.
