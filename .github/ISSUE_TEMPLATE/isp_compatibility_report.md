---
name: "🇹🇷 ISP Compatibility Report"
about: Report a working or failing Discord bypass strategy for your Turkish ISP
title: "[ISP Report] <ISP Name> - <City/Region>"
labels: ["isp-compatibility", "triage"]
assignees: ""
---

### 📡 Network Details
- **ISP Name**: (e.g. Turkcell Superonline, Türk Telekom, TurkNet, Vodafone, Türksat Kablonet, NetSpeed, Millenicom)
- **Connection Type**: (e.g. Fiber GPON, VDSL, Cable, 4.5G)
- **City / Region**: (e.g. Istanbul, Ankara, Izmir, etc.)
- **Autonomous System Number (ASN)**: (Shown in `discord-bypass diagnose`, e.g. AS34984, AS9121)

---

### ⚡ Auto-Tuner Outcome
- Did you run `sudo discord-bypass tune`? [Yes / No]
- **Selected Strategy**: (e.g. `strategy_c`, `strategy_c_ttl5`, `strategy_split2`, etc.)
- **Verification Status**: [Verified / Failed]

---

### 📝 Diagnostic Summary
Please paste the summary section from `discord-bypass diagnose`:

```text
(Paste output here)
```

---

### 🎮 Discord Client Testing
- [ ] Discord Desktop Client connects cleanly
- [ ] Discord Web (`discord.com/app`) connects cleanly
- [ ] Text channels and images/media load
- [ ] Voice channels connect (RTC Connecting -> Connected)
