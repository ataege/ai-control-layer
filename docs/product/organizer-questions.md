# Organizer questions (RS-01)

Owner: researcher, document owner and presenter. Status: **answer 2 (deadline) received; 1, 3, 4 open.** The lead sends the
message; the answers are recorded below, verbatim, when they arrive.

Sources: the competition rules, `competition-rules.pdf` ([S9]), and the challenge criteria,
`competition-criteria.pdf` ([S10]). The questions follow the report's list "Questions to resolve with
organizers and sponsor mentors" ("Research documentation and submission workflow"). The report asks
us to "Preserve the exact organizer answer and its source; do not infer permission from common
hackathon practice."

## Message to send

Paste the text below as it stands. Question 5 is optional: delete it if the team is not considering
another track (RS-02).

> Hello, we are the Task Passport team in the AI Control Layer challenge. Before we go further we
> would like to confirm five points about the rules and the criteria:
>
> 1. **Preparation before the start.** Rule 5 says teams must have "started solving the competition
>    task no earlier than" the start time. Criteria section 5 says pre-existing "agents, applications
>    and other unrelated components will not be subject to assessment". Before the start we prepared:
>    (a) a generic monorepo starter (Next.js, NestJS, Go, PostgreSQL), written with AI assistance on
>    2 October 2026, containing only health checks, configuration, an empty database setup and an
>    authentication placeholder that rejects every request, with no control-layer features; and
>    (b) a written design report, architecture diagrams and a task plan for the control layer.
>    May we use these? If yes, how should we disclose them: in the project description, in the
>    presentation, in the repository README, or elsewhere?
> 2. **Start and deadline.** The rules print "11:00 PM on October 3rd" for the start and "11:00PM on
>    October 4th" for the submission. Could you confirm the intended local times for both?
> 3. **Scoring weights.** The rules (section 11) give 20% to the self-testing suite and 10% to
>    practical implementability and scalability. The criteria (section 8) give 15% to each. Which
>    weights apply in phase 1?
> 4. **Judge access and presentation.** The criteria (section 6) say judges run our test suite,
>    submit ad-hoc prompts and edit our configuration files. The criteria (section 7) ask us to use
>    local models, for example through Ollama. How will phase-1 mentors reach the running layer:
>    will they clone and run the repository themselves (if so, on what operating system and
>    hardware), test it on our machine, or expect a hosted link or a recording? Is a synthetic
>    invoice workflow with a simulated message outbox (no real email) acceptable as the demonstration
>    agent? How long is the live pitch in phase 2?
> 5. **(Optional) Cross-track.** Are cross-track submissions allowed, and may one project, or parts
>    of it, be reused in another track?
>
> Thank you.

## Why each question is asked

| #   | Question                      | Source of the doubt                                                                                                                                                                                                                                                                                                                                                                                           | Open item it closes                                            |
| --- | ----------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------- |
| 1   | Preparation before the start  | Rules section 5 (page 1); criteria section 5 (page 3). Report: "The criteria allow pre-existing agents, applications and unrelated components [S10, section 5], but do not expressly resolve advance work on the assessed control layer or task-specific planning." `docs/preparation-record.md` describes the starter and states that it "was written with AI assistance, using Claude Code, on 2026-10-02". | Decision 8 (`docs/product/README.md`); X-01                    |
| 2   | Start and deadline            | Rules section 5 (page 1) prints both times as PM. The lead confirmed 11:00 AM; the report asks us not to "silently reinterpret PM as AM", so the question does not assume an answer.                                                                                                                                                                                                                          | `start time confirmation`                                      |
| 3   | Scoring weights               | Rules section 11 (page 2): 30/20/20/20/10. Criteria section 8 (page 4): 30/20/20/15/15. Report: "The team should request clarification rather than assert precedence."                                                                                                                                                                                                                                        | `scoring weights`                                              |
| 4   | Judge access and presentation | Criteria section 6 (page 4): judges "execute the automated test suite", use "spontaneous, ad-hoc prompts" and "may modify the configuration files/feeds". Criteria section 7 (page 4): local models, no paid subscriptions. Rules section 8 (page 2): phase 1 evaluates submissions on HackTribe, phase 2 is live pitching by finalists. Neither document gives the pitch length.                             | `judge access`; presentation length for RS-04, RS-08 and SH-33 |
| 5   | Cross-track (optional)        | Neither the rules nor the criteria mention other tracks. Report: "If entering another track, confirm whether cross-track submission and reuse are permitted; this report establishes no such permission."                                                                                                                                                                                                     | RS-02 (dropped if no other track is considered)                |

## HackYeah FAQ evidence, found after the message was drafted

The general HackYeah FAQ (verbatim in [requirements.md](requirements.md), "HackYeah FAQ") bears on two
questions. It is evidence, not an answer for this task:

- Question 1: F-13 says "If you wish to use any previously completed or external resources, tools,
  repositories, etc., you must fairly cite or note that in your presentation, code, etc." The AI
  Control Layer rules' "started solving the competition task no earlier than" wording is stricter, so
  the question stays: the organizers confirm for this task. Whatever they answer, the starter and the
  design documents are disclosed with their true preparation dates.
- Question 5: F-4 and F-5 say a team may "submit only one project in each category" and "we strongly
  discourage submitting one project to more than one category." If the message has not been sent,
  question 5 can be deleted; the team submits this project to this category only.

## Answers

Record each answer verbatim with its date, channel and the person who answered. Until an answer is
recorded the question stays open, and nothing in the presentation or submission assumes its outcome.

| #   | Date and time                                    | Channel                                           | Answered by                                                | Verbatim answer                                                                                                                                                                                                                                                                                                                                                                                                                                                                           | Consequence                                                                                                                                                                                                                                                        |
| --- | ------------------------------------------------ | ------------------------------------------------- | ---------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| 1   | not yet asked                                    |                                                   |                                                            |                                                                                                                                                                                                                                                                                                                                                                                                                                                                                           |                                                                                                                                                                                                                                                                    |
| 2   | 3 October 2026, about 19:30 (pasted by the lead) | Relayed by the lead; sender and channel not given | Organizers (an announcement about the first project draft) | "⏰ The deadline for the first project draft is getting closer! Please remember that selecting the "Published" option in your project does not matter at this stage. Your first draft should mainly include: a few sentences about the project you are working on, information about the category you are competing in. Clicking Submit does not mean that your work is finished. After submitting the draft, you can still freely edit and update your project until 11:00 AM tomorrow." | Editing closes at 11:00 AM on 4 October 2026: the deadline half of `start time confirmation` is settled. A first draft (a few sentences and the category) is due now; see the submission checklist. The start time is not covered; it stays 11:00 AM per the lead. |
| 3   | not yet asked                                    |                                                   |                                                            |                                                                                                                                                                                                                                                                                                                                                                                                                                                                                           |                                                                                                                                                                                                                                                                    |
| 4   | not yet asked                                    |                                                   |                                                            |                                                                                                                                                                                                                                                                                                                                                                                                                                                                                           |                                                                                                                                                                                                                                                                    |
| 5   | not yet asked                                    |                                                   |                                                            |                                                                                                                                                                                                                                                                                                                                                                                                                                                                                           |                                                                                                                                                                                                                                                                    |

## Recording the answers elsewhere

RS-01's "Done when" puts decision 8's answer in `docs/product/README.md` and the required disclosure
in `docs/preparation-record.md`. Those files are outside this track's paths (the README is shared with
the Go implementer; `docs/preparation-record.md` belongs to integration), so the researcher sends the
proposed text to the lead, who lands it. Proposed disclosure, to adjust to the organizers' answer:

> Before the official start we prepared a generic starter (health checks, configuration, an empty
> database setup and an authentication placeholder; no control-layer features), written with AI
> assistance on 2 October 2026 and described in `docs/preparation-record.md`, and a design report,
> architecture diagrams and a task plan. The repository's first commit (`3016ed4`) contains that
> starter; the control-layer work is in the later commits.

Checked against the git history on 3 October 2026: `3016ed4` ("initial commit", 11:12 +0200 on
3 October) adds the whole starter in one commit (206 files), so its timestamp is the upload time, not
the time the starter was written; the preparation date comes from `docs/preparation-record.md`. The
report and the diagrams entered the repository in `d801496` (11:43) and later commits. Before the
disclosure is used, recheck that no control-layer code predates the confirmed start.
