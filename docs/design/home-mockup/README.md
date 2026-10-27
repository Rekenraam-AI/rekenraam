# Home mockup (v3)

Source of the interactive mockup behind the
[Home and navigation design](../home-and-navigation.md), saved 2026-10-27
after the owner approved the third round. All amounts, payees and accounts
are sample data.

These files are the source of a Claude Design canvas, not application code.
Each `.dc.html` file is one artboard and expects the canvas runtime
(`support.js`), so it does not open as a normal web page. The published
canvas is
[Rekenraam Home mockup](https://claude.ai/artifact/5gVAfVjNiSMRt5hBSRwCsQ)
(private to the owner unless shared).

| File | Shows |
|---|---|
| `Main.dc.html` | Desktop Home, interactive: briefing, command bar, shortcuts, My Home, More to explore, Customize panel, Add transaction dialog |
| `Phone.dc.html`, `PhoneScrolled.dc.html` | The same page at 390px: first screen and full scroll |
| `Customize.dc.html`, `Palette.dc.html`, `AddTransaction.dc.html` | Main opened with the Customize panel, the command bar dropdown, or the Add transaction dialog |
| `Dark.dc.html` | Dark theme |
| `MissingPrice.dc.html` | Headline when a price is missing |
| `canvas.json` | Artboard layout and review notes |

All other artboards import `Main.dc.html` with different start states, so
`Main.dc.html` is the one to edit. Colours follow the light and dark tokens
in `frontend/src/app.css`; the type face is Inter.
