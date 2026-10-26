# Engineering Principles

Oct 19, 2026&#32;

## Purpose

Humans own the architecture, design and direction; agents write the code. These rules apply in every language and project, and where a repository's conventions disagree, the repository wins.

- **Code is a liability.** Every line must be read, maintained, and may be wrong, so write less of it.
- **Modular and abstract by design.** Build small parts behind clear interfaces around the general mechanism, not the first use case. Draw boundaries up front; add features only when needed.
- **The next reader has no context.** Write code, docs and logs for someone who was not there.

## For humans

You supply what an agent cannot: intent, structure and judgment. You are accountable for what ships.

1. **Own the why.** Write down what the project is for, what it is not, and its one-sentence principle.
2. **Insist on modular, abstract design.** This rule matters most: require named modules, the interface between each, and the general concept each implements. Reject any part that cannot be replaced or tested alone.
3. **Approve a written design before code.** You choose the boundaries, dependencies and data shapes; agents write the design up and wait for your yes.
4. **Challenge complexity,** including in your own requests. Prefer the simplest design that survives production.
5. **Delegate outcomes, one concern at a time.** State what done looks like, how it is verified and what must not change.
6. **Review for design, not syntax.** Check the diff against the request and the agent rules below; reject growth beyond the ask.
7. **Demand evidence.** Accept "done" only with run output, test results or numbers.
8. **Own the paper trail.** Make sure the files listed under Process and delivery stay current, and state whether agents may commit or push.
9. **Decide what matters, quickly.** Agents make routine calls; be reachable when readings diverge materially, and triage the side issues they report.
10. **Fix the rule, not the instance.** When an agent repeats a mistake, change this doc, the entry file or the design.

## For agents

These rules bind whoever writes the code. Humans enforce them in review.

### Design and structure

Abstract the boundaries now; build features only when they are needed.

- **Design to interfaces.** Each module exposes a small, named surface and hides its internals; callers use only that surface.
- **Abstract the mechanism, keep the instance concrete.** Name the general concept (a source, a policy, a transport) and implement only today's case behind it.
- **Keep the core generic.** Domain names live in one edge file and each third-party system in one adapter, so a swap touches one file.
- **Layers point down.** Lower layers know nothing of upper ones; cross a boundary only through a named function.
- **Small modules, one purpose each.** Prefer composition and plain data flow over inheritance.
- **No speculative features.** Generalize behavior only after a pattern appears twice; no class where a function will do.
- **Every dependency justifies itself.** Prefer the standard library; never add a library or build step to save a few lines.
- **Say it once.** Knowledge that must agree lives in one place: parameterize instead of forking, and generate anything derived. Lines that look alike but change for different reasons stay separate.
- **One representation per concept,** carried end to end. Metadata wraps the object; it never lives inside it.
- **Match the repo.** Follow its existing patterns and extend an existing abstraction before adding one; leave untouched code unformatted.
- **Delete rather than deprecate.** No dead branches, unused flags or commented-out code.
- **Prefer the boring option:** a monolith over a distributed system, explicit code over magic.

### Clarity

Aim for obvious, not clever. Output is an interface too: people, programs and models read it.

- **Names carry the meaning.** Functions are verbs, values nouns, booleans assertions (`expired`); no `process`, `data`, `util` or private abbreviations.
- **Use the problem's words,** not the container's.
- **One function, one job.** If it needs "and" to describe it, split it; past three levels of nesting, return early or extract.
- **Every number gets a name,** grouped so a module's tunables are visible in one place.
- **Explicit setup.** No hidden initialization or global state.
- **Comment only what code cannot say:** an invariant, a rejected obvious approach, an external workaround, an ordering constraint. Never narration.
- **Aggregate and cap output.** Group repeats with a count, cap long lists, and return only what the reader needs.
- **Keep machine-readable shapes stable.** Add fields freely; never rename them.
- **Say what happened,** not which function ran, and say when something degraded or fell back.
- **Docs lead with the answer.** Short sentences, plain words, numbers over adjectives.

### Correctness

No credit for code that merely looks right.

- **One error path.** Raise a typed error where the failure happens; turn it into a message in one place per boundary.
- **Fail fast and loudly.** Never ignore a failure, and never swallow one to turn a test green.
- **Errors say what would work:** `expected a number from 1 to 12`, not `invalid input`. No stack traces reach users.
- **One discipline for shared state.** Never mix synchronous and asynchronous access to the same data.
- **Bound everything that grows.** Queues, caches and maps have limits; a full queue drops the oldest item.
- **Periodic work gets its own scheduler,** timed against a monotonic deadline, not repeated sleeps.
- **Run it.** Execute the path you changed, and re-run the check for your risk after nearby changes.
- **Numbers behind every performance claim.** Measure before optimizing; compare against a baseline with repeats, and report variance and the ceiling.
- **Test where bugs live:** wiring, concurrency, serialization and contracts. Handle every enum case so a new one fails loudly.
- **Make tests fast and repeatable.** Inject the clock, seed randomness and shrink intervals.
- **No vacuous passes.** A test must fail if the feature did nothing.
- **Report honestly.** Show failing output, name skipped steps, and never call unverified work done.

### Process and delivery

The repository alone must be enough for a new session to continue.

- **Design before code.** Beyond a small change, write what it does, the data flow, the components, the stages and what is out of scope.
- **State the principle in one sentence.** If it will not fit, it is not understood yet.
- **Build in stages.** Each stage proves one thing; ship the minimal version, then refactor from real use.
- **Log decisions as they are made:** what was chosen, why it beat the alternatives, and the rejected approaches and root causes.
- **Keep three files current.** A README (what it is; build, run, test), an entry file (`CLAUDE.md` / `AGENTS.md`) pointing to it and this doc, and a running log.
- **Docs change with the code,** in the same commit, and state what they cover.
- **Deliver what was asked.** No quiet narrowing or widening; finish every part and name what was blocked.
- **Flag adjacent problems; don't fix them** in the same change.
- **Disagree once, then commit.** State the objection briefly; if the decision stands, do it fully.
- **Make routine calls yourself.** Ask only when readings lead to materially different work.
- **Commit small, saying what and why.** One idea per commit that builds and passes: `parser: read length from the header`, not `fix`.
- **Nothing generated or secret is committed.** Review what you stage; ship a `.env.example` and keep real values local.
- **One command builds, runs and tests,** in a pinned, reproducible environment.
- **Containerize when the environment matters.** Use one when the software has system dependencies, runs as a service, or must behave the same everywhere. Pin versions, run as non-root, and keep secrets out of the image; skip it when a plain script or library needs none.
- **Version what others install,** with a changelog, newest first.
- **Follow the repo's commit policy.** Where review comes first, leave changes in the working tree.
