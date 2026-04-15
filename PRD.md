# PRD
## Problem Statement
Users currently have to sign in via the existing legacy flow before seeing dashboard content. This results in:
- **30 %** of users abandoning the onboarding flow after the email step.
- **$X** in lost revenue per day due to bounced users.
- **Long loading times** (up to 5 s) before the dashboard is visible.

**Goal:** Reduce abandonment before the dashboard by collecting the email earlier, streamlining the flow, and increasing perceived value.

## Scope & Features
- **New Onboarding Component**: a full-page UI that collects a valid email address.
- **Single‑Page Flow**: Users can enter their email once and view the main dashboard without an extra sign‑in step after the first visit.
- **OTP Verification (optional)**: A phone verification step that can be toggled on or off per environment.
- **Responsive Design**: Works on all modern browsers, adapts to iPad, MacBook, and iPhone XS and later. No support for iOS 12 or Chrome < 59.
- **SEO**: The onboarding page is SEO‑friendly and does not block crawlers.
- **Analytics**: Send an event to the analytics system on email submission; track conversion rates.
- **Fallback to Legacy Flow**: For non‑mobile or if email fails, automatically redirect to the legacy login.

## Data Flow
1. **Landing at** `/welcome` **Route**: The app serves the onboarding component.
2. User enters a valid email → the frontend posts a JSON request to `/api/onboard`.
3. **Auth API** validates the email:
   - If **new user**, creates an account, sends a welcome email, and issues a session cookie.
   - If **existing user**, verifies the account, updates last‑seen, and issues a session cookie.
4. The backend returns a response with the session cookie set via `Set-Cookie` headers.
5. The frontend receives the cookie (browsers set it automatically) and redirects the user to `/dashboard`.
6. The dashboard is pre‑loaded with the latest user data; no further login is required.

## Edge Cases & Risk Assessment
**A. Expired Link & Rate‑Limiting**
- **Risk**: Users may attempt to use the same email too frequently, triggering rate limits.
- **Mitigation**: Implement server‑side rate limiting (5 req/min) and client‑side cooldown UI.
- **Testing**: Simulate repeated email submissions and verify that the correct error response is shown.

**B. Duplicate Email**
- **Risk**: A user tries to onboard with an email already in use.
- **Mitigation**: Return a 409 conflict; frontend shows a message and offers to sign in instead.
- **Testing**: Create a duplicate test flow and confirm the UI prompt appears.

**C. Network Failures & Retries**
- **Risk**: Users on flaky connections lose progress.
- **Mitigation**: Exponential back‑off in the client retry logic; fallback to legacy flow if the network is unavailable.
- **Testing**: Force network failures via dev tools or mock the API.

**D. OTP Failure / Phone Confirmation**
- **Risk**: If the user opts into OTP, a failed verification may leave them stranded.
- **Mitigation**: Provide a “Try again” button and a “Skip phone” option in a secondary flow.
- **Testing**: Simulate OTP failures, network errors, and rate limits.

**E. UI / Browser Compatibility**
- **Risk**: CSS or HTML quirks break the onboarding layout on older browsers.
- **Mitigation**: Test on iOS 14+, iOS 13 in 256‑dp display, Chrome 60+. No support for older browsers.
- **Testing**: Run `bcd` or `browserstack/local` across the target range.

**F. Legacy Flow Fallback**
- **Risk**: Errors in detecting failures may redirect users to a broken legacy flow.
- **Mitigation**: Gracefully handle API errors, show a friendly “Something went wrong; try again” message.
- **Testing**: Simulate 500 errors from the auth API and confirm the app returns a 500 page with a retry option.

## Test Strategy
| Scenario | Type | Test
|----------|------|------
| Happy Path | Manual + Unit | Smoke test: email → dashboard | 1
| Duplicate Email | Manual + Integration | Submit an already‑used email, expect conflict and sign‑in prompt | 1
| Rate Limit | Integration | Rapidly submit > 5 emails per minute, expect 429 response and cooldown UI | 1
| OTP Success | Integration | Complete OTP flow and verify session cookie | 1
| OTP Failure | Integration | Simulate failing OTP and confirm retry / skip options | 2
| Network Failure | Integration | Disconnect network mid‑submit, confirm retry logic | 2
| Page Load Performance | End‑to‑end | Measure time to interactive and first paint | 2

**Unit Tests**:
- Validate email format checker.
- Validate API payload serialization.
- Validate rate‑limiting logic.
- Validate error handling logic.

**Integration Tests**:
- Use the `app.ts` test harness to run the onboarding flow against the real API in a staging environment.

## Deployment Notes
- New feature flag: `NEW_WELCOME_ONBOARDING=true`. Deploy the backend change, then enable the flag via environment variables in the staging and prod configurations.
- The frontend change lives in the `feature/onboarding-new` component. We will commit it into the `dev-branch`. A `feature-wel-onboarding` PR will be created.
- We will target a **feature flag rollout** to a 5 % slice before a full A/B test.

## Documentation Update
- Update `docs/design.md` with the new wireframes.
- Add new `components/Onboarding.tsx` to the storybook.
- Update `CHANGELOG.md` with the new flow and API changes.

## Release Plan
1. **Merge PR** on DEV when unit & integration tests pass.
2. **Create PR** to feature flag enablement.
3. **Run performance tests** on staging.
4. **Do a 5 % A/B rollout** in prod.
5. **Monitors**: Page Load, bounce rate on `/welcome`, email capture rate, OTP success rate.

**Rollback**: If the churn spikes, immediately cut the feature flag.

## Owner & Stakeholders
- **PO**: [Your name]
- **Lead Engineer**: [Dev name]
- **UX Lead**: [UX name]
- **Marketing**: Coordinate SEO and messaging.
- **Analytics**: Set up events for email submit, OTP success, conversion.

---

**Design System Preview**
The design system mockups are available at:
```bash
open ~/.gstack/projects/$(git rev-parse --abbrev-ref HEAD)/designs/design-system-$(date +%Y%m%d)/variant-<CHOSEN>.png
```
- Open each file to preview the look. Adjust colors or typography as needed before finalizing the PR.

**Next steps**:
- Let me know which style variant you prefer.
- Verify that the PRD is complete. If you want to add another risk or tweak scope, let me know.
- We'll generate a new PR once the design system is finalized.

**Done**
```
# End of PRD
```