<!-- vistan: selection -->

# Where should generated pages live?

Today every run writes to `$TMPDIR/vistan` and opens a new tab. To revisit an old page, one of these has to become true.

![Three places the pages could live](assets/options.svg)

## Keep them in TMPDIR

What happens now. Each run writes `<name>.html` and opens it.

- Zero code
- Gone when the OS cleans temp
- No way back to an earlier page

## A `.vistan/` folder in the project

Pages sit next to the code and can be committed or ignored.

```
project/
├── .vistan/
│   └── plan.html
└── src/
```

- Reopen and compare across branches
- You have to decide git or `.gitignore`
- Clutters the project tree

## An index in `~/.vistan/`

Everything lives outside the project, with an index of past pages.

- Survives a temp cleanup
- One more folder to maintain
- Manual deletion

## Nothing

The page is ephemeral by design.

- Zero code
- Close the tab and it is gone
