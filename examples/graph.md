<!-- vistan: graph -->

# How a review reaches a decision

## Context

Three reviewers look at one change. The graph shows what each step produces and where the loop closes: the human's decision goes back to the orchestrator, which is why the last edge points upward.

## Graph

```
orchestrator --> reviewers
reviewers --> review.md : findings
review.md --> vistan : render
vistan --> human : page
human --> orchestrator : Apply F1, F3
vistan --> history [History]
```

## Reading

- Every node is a noun: a process, a file or a person. The edge label is what travels.
- The one edge pointing back up is the part the page does not finish: the choice goes back to the local server, and the agent reads it.
- `history` hangs off the page because the same render also keeps it, and that is the only branch that outlives the run.
- What the graph leaves out: the severity vocabulary, the checks, and the code each reviewer read.
