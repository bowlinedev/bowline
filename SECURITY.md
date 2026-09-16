# Security policy

## Supported versions

| Version | Supported |
| --- | --- |
| 1.0.x | yes |
| < 1.0 | no |

Support windows for the 1.0 line and later are described in `docs/lts.md`.

## Reporting a vulnerability

Email tunardev@gmail.com. Once this repository is public, GitHub's [private vulnerability reporting](https://github.com/bowlinedev/bowline/security/advisories/new) is the preferred channel; GitHub offers that form only on public repositories.

Please include the affected version, a description of the impact, and the smallest reproduction you have. A proof of concept helps but is not required.

Do not open a public issue for a vulnerability.

## What to expect

- Acknowledgement within three business days.
- An assessment with a severity and a planned fix date within ten business days.
- A fix released within ninety days of the report, sooner for a high or critical severity.
- Credit in the advisory unless you ask otherwise.

## Disclosure

Disclosure is coordinated. The advisory is published when the fix is released, together with the affected versions, the fixed versions, and a workaround if one exists. If a report is already public or under active exploitation, the fix and the advisory go out as soon as they are ready.

## Scope

In scope: the runtime module, the generators and the code they emit, the gateway, the registry, the signing package, the MCP server, and the client runtimes published from this repository.

Out of scope: the example applications under `examples/`, vulnerabilities in dependencies of those examples, and findings that require an attacker to already hold the server's signing secret or a valid session.
