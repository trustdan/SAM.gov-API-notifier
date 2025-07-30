**Short answer — yes.** SAM.gov sits behind *api.data.gov*, which enforces fairly aggressive quotas.
If you trip those limits your key is “quarantined” for the rest of the **rolling hour *or* calendar‑day window**, so it’s completely normal to keep seeing HTTP 429 the following morning.([api.sam.gov][1], [api.sam.gov][2])

Below is what’s happening, why the block can last longer than you expect, and the work‑arounds teams typically use.

---

## 1  How the SAM.gov throttling actually works

| Scope                           | Effective limit\*                                                                                                                                                                                                                 | What happens when you exceed it                                                                                               |
| ------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------- |
| **Per‑hour (rolling)**          | **1 000 requests/hr per key** for most SAM services([api.sam.gov][2])                                                                                                                                                             | Key is suspended until one full hour has elapsed since the first request that breached the quota.                             |
| **Per‑day (calendar‑day, UTC)** | Public “non‑federal” keys on some endpoints are capped at **10 calls/day** (e.g., PSC API)([open.gsa.gov][3]); similar undisclosed caps exist on Opportunities and Entity APIs (“federal” keys get 1 000/day).([open.gsa.gov][4]) | Key remains blocked until 00:00 UTC. Users in U.S. time zones often find it’s still “tomorrow” locally when the block clears. |
| **Global abuse guard**          | Undocumented; triggers if you burst many parallel calls or hammer multiple *api.data.gov* services at once.([Data.gov][5])                                                                                                        | Key or source IP is put in a longer‑lived deny list; only the help desk can clear it.                                         |

\*All requests made **with the same key** across *any* api.data.gov‑hosted API count toward the same counters.([api.sam.gov][1])

---

## 2  Why a “twice‑a‑day” script can still be blocked the next day

1. **Pagination multiplies your call count** – a single job ad search at `limit=1000` can return dozens of pages; each page is a separate request that the quota tallies.
2. **UTC reset** – if you run at, say, 23:30 and 05:30 CT, both executions occur before 05:00 UTC and land in the *same* calendar‑day window.
3. **Other SAM endpoints** – the hourly/daily counters aggregate calls you (or a teammate) make to PSC, Exclusions, UEI, etc. behind the scenes.
4. **Key rotation & terms‑of‑use** – keys more than 90 days old or not rotated when SAM auto‑generates a replacement are silently throttled.([dodprocurementtoolbox.com][6], [SAM.gov][7])

---

## 3  Recommended mitigations

### A. Reduce call volume

* **Use narrower date ranges** and the `updatedTo/From` or `postedFrom/To` parameters so you only pull deltas.
* **Leverage pagination efficiently**: request the **maximum `limit` (1000)** so each job uses the fewest calls.
* **Cache unchanged records** locally and skip re‑downloads.

### B. Add polite client‑side throttling

* Implement **exponential back‑off with jitter** on 429/500/502/503 responses (see API docs & general guidance).([Grants.gov][8], [api.sam.gov][1])
* Serialize requests (1–2 rps) rather than firing them in parallel.

### C. Time your checks after the UTC reset

Running once after 01:00 UTC (\~20:00 CT the previous evening) and again after 13:00 UTC (\~08:00 CT) keeps each run in its own quota window.

### D. Ask for a higher ceiling

The *api.data.gov* team explicitly invites heavy users to request elevated limits.([api.sam.gov][2], [api.sam.gov][1])
Open a ticket with the **Federal Service Desk** (FSD) and justify your use case or request a **system account** (federal keys get 1 000/day by default).([alpha.sam.gov][9])

### E. Monitor & rotate keys

* Watch response headers: many endpoints include `X-RateLimit-Remaining` and `Retry-After`.
* Rotate your key when SAM notifies you; stale keys are throttled and eventually disabled.([dodprocurementtoolbox.com][6])

---

## 4  Key references

1. api.data.gov rate‑limit page (1 000 rph, rolling‑hour logic)([api.sam.gov][2])
2. PSC Public API – 10 calls/day non‑federal limit([open.gsa.gov][3])
3. Opportunities API – daily limits vary by role([open.gsa.gov][4])
4. Error code 429 definition in SAM docs([api.sam.gov][10])
5. General rate‑limit error guidance & “contact us” invitation([api.sam.gov][1])
6. api.data.gov manual – rolling‑hour example([Data.gov][11])
7. api.data.gov service overview & abuse‑guard rationale([Data.gov][5])
8. GSA developer TOS – keys may be “permanently or temporarily blocked” for circumvention([U.S. General Services Administration][12])
9. SAM help page – links to rate‑limit FAQ & system‑account guide([SAM.gov][13])
10. Federal Service Desk contact channel for SAM API issues([alpha.sam.gov][9])

---

### Bottom line

The behavior you’re seeing isn’t a bug; it’s the SAM/api.data.gov safeguard doing exactly what it’s designed to do. Stay under the documented caps (or request a higher tier), back‑off gracefully on 429s, and schedule your two daily syncs in separate UTC days, and you should stop running into the lingering block.

[1]: https://api.sam.gov/docs/rate-limits/?utm_source=chatgpt.com "Web Service Rate Limits | Your API Site Name"
[2]: https://api.sam.gov/docs/rate-limits/ "
        Web Service Rate Limits | 
      Your API Site Name
    "
[3]: https://open.gsa.gov/api/PSC-Public-API/ "SAM.gov PSC Public API | GSA Open Technology"
[4]: https://open.gsa.gov/api/get-opportunities-public-api/ "SAM.gov Get Opportunities Public API | GSA Open Technology"
[5]: https://api.data.gov/about/?utm_source=chatgpt.com "About api.data.gov"
[6]: https://dodprocurementtoolbox.com/uploads/System_Account_User_Guide_v3_01_5f66649acf.pdf?utm_source=chatgpt.com "[PDF] System Account User Guide - DoD Procurement Toolbox"
[7]: https://sam.gov/about/terms-of-use?utm_source=chatgpt.com "Terms of Use | SAM.gov"
[8]: https://grants.gov/api/status-codes?utm_source=chatgpt.com "Status Codes | Grants.gov"
[9]: https://alpha.sam.gov/help?utm_source=chatgpt.com "Help - SAM.gov"
[10]: https://api.sam.gov/docs/errors/?utm_source=chatgpt.com "General Web Service Errors | Your API Site Name"
[11]: https://api.data.gov/docs/developer-manual/?utm_source=chatgpt.com "Developer Manual - Data.gov's API"
[12]: https://www.gsa.gov/technology/government-it-initiatives/digital-strategy/terms-of-service-for-developer-resources?utm_source=chatgpt.com "Terms of Service for GSA.gov's Developer Resources"
[13]: https://sam.gov/help?utm_source=chatgpt.com "Help - SAM.gov"

