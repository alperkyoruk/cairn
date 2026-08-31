# Positioning and the landing page

Read `docs/agents/shared-context.md` first — it is the other half of this brief.
Write your report to `docs/agents/out/10-positioning.md`.

You write the page that decides whether a developer tries this or closes the tab.
Cairn is an open-source project with no growth budget, no ads, and no audience —
the only distribution is that the idea is interesting enough to pass between
people.

**Read first:** `site/index.html` and `site/style.css` (the current landing page,
published to GitHub Pages), `README.md`'s opening and "Why" sections, and
`docs/design_handoff_cairn_site/` (the design the page was built from).

## The job

1. **Name the idea in one sentence** that is true, specific, and not a category
   claim. "An issue tracker for AI agents" is a category claim and every reader
   has already decided what it means before they finish it. The real claim is
   narrower and stranger: an agent physically cannot leave a task without saying
   where it left off, and two decisions are withheld from it by the state
   machine. Find the sentence that makes a reader who has been burned by an agent
   quietly abandoning a half-finished task recognise their own experience.

2. **Audit the current page against the fifteen seconds it gets.** What is above
   the fold, what is the reader's first question, and does the page answer that
   question or a different one? Where does it explain when it should show?

3. **Find the demonstration.** This product's thesis is visible in an artifact: a
   real state block and a real worklog with dead ends in it, or the exact text of
   the refusal an agent gets when it tries to mark its own work done. One of those
   on the page is worth three paragraphs of explanation. Choose one and write it
   out with real content.

4. **Identify who this is genuinely for and who it is not for**, and say the
   second part on the page. A tool that says "not for you if you have a team"
   earns the trust of the person it is for. The closed out-of-scope list is a
   positioning asset, not an apology — treat it as one.

5. **Check the page works:** offline-safe assets, dark mode, narrow window, the
   OG image and meta tags, and that every claim on it matches what the software
   actually does today.

## Do not

- Do not write in marketing register: no "supercharge", no "seamlessly", no
  feature-benefit grid, no testimonial section, no pricing table, no waitlist.
- Do not claim anything the software does not do today.
- Do not add analytics, a tracking pixel, an email capture, or an embedded font.
  The site is static, offline-safe, and hosted on GitHub Pages.
- Do not compare Cairn to named competitors by name. The comparison the page
  makes is with a habit, not a product.

## Deliverable

The one-sentence claim, with two rejected alternatives and why they are worse.
Then a section-by-section critique of the current page. Then the rewritten copy
in full, ready to paste, with the demonstration artifact written out. Then the
technical checklist results.
