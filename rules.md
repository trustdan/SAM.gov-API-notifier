# Vibe Coding Process Rules & Guidelines

## 🎯 Core Philosophy
"Progress with purpose, document with discipline, code with clarity"

## 📍 Rule 1: Roadmap Position Tracking

### 1.1 Start Each Session
```markdown
## Session: [DATE] [TIME]
### Current Position
- [ ] Roadmap Step: [X of Y] - [Step Name]
- [ ] Previous Session Ended At: [Specific point]
- [ ] Today's Target: [Clear goal]
```

### 1.2 Progress Log Format
```markdown
### Progress Log
- ✅ Completed: [What was finished]
- 🚧 In Progress: [What's being worked on]
- 🔮 Next Up: [What comes after this]
- ⏰ Time Spent: [Actual vs Estimated]
```

### 1.3 End Each Session
```markdown
### Session End: [TIME]
- Last Line of Code: [File:Line]
- Mental State: [What you were thinking/solving]
- Next Session Should Start With: [Specific action]
```

## 🐛 Rule 2: Issue & Blocker Documentation

### 2.1 Issue Logging Template
```markdown
### Issue #[NUMBER]: [Brief Description]
**Encountered At:** [Timestamp] | [File/Function]
**Category:** 🔴 Blocker | 🟡 Warning | 🟢 Note
**Description:** [What happened]
**Expected:** [What should have happened]
**Actual:** [What actually happened]
**Solution:** [How it was resolved] | ❓ Pending
**Time Lost:** [Minutes/Hours]
**Lesson Learned:** [Key takeaway]
```

### 2.2 Blocker Escalation
- 15 min rule: Try solving for 15 minutes max
- Document attempted solutions
- Tag for help if needed: `@help-needed`
- Move to parking lot if not critical path

## 💭 Rule 3: Decision Documentation

### 3.1 Decision Log Format
```markdown
### Decision: [What was decided]
**Context:** [Why this decision was needed]
**Options Considered:**
1. Option A: [Pros/Cons]
2. Option B: [Pros/Cons]
**Choice:** [What was chosen]
**Rationale:** [Why this option]
**Reversible:** Yes/No
**Review Date:** [When to revisit]
```

### 3.2 Technical Debt Tracking
```markdown
### Tech Debt Item: [Description]
**Introduced:** [Date/Commit]
**Reason:** [Why the shortcut was taken]
**Impact:** Low | Medium | High
**Fix Effort:** [Estimated hours]
**Fix By:** [Target date]
```

## 🔄 Rule 4: Development Flow Rules

### 4.1 Commit Message Standards
```
[TYPE][SCOPE]: Brief description (max 50 chars)

- What: [What changed]
- Why: [Why it changed]
- Impact: [What this affects]

Roadmap: Step X.Y completed/in-progress
Issues: Closes #N, References #M
```

Types: `feat|fix|docs|style|refactor|test|chore`

### 4.2 Branch Naming
```
feature/roadmap-step-X-brief-description
fix/issue-N-brief-description
experiment/idea-brief-description
```

### 4.3 Code Comment Rules
```python
# TODO(@you, by:YYYY-MM-DD): Clear action item
# HACK: Explanation of why this hack exists + issue#
# FIXME(priority:high|med|low): What needs fixing
# NOTE: Important context for future you
# QUESTION: Uncertainties to resolve
```

## 📊 Rule 5: Progress Visualization

### 5.1 Daily Status Update
```markdown
## Daily Status: [DATE]
### Momentum: 🔥 On Fire | ✅ Steady | 🐌 Slow | 🛑 Blocked

### Completed Today:
- [x] Task 1 (Est: 2h, Actual: 1.5h) ⚡
- [x] Task 2 (Est: 1h, Actual: 3h) 🐌

### Velocity Score: [Actual/Estimated]
### Blocker Impact: [Hours lost to blockers]
```

### 5.2 Weekly Retrospective
```markdown
## Week of [DATE]

### Wins 🎉
- [Major accomplishments]

### Challenges 😤
- [What was difficult]

### Learnings 📚
- [New knowledge gained]

### Process Improvements 🔧
- [What to change next week]
```

## 🧪 Rule 6: Testing & Validation

### 6.1 Test Creation Rules
- Write test BEFORE fixing bug
- Document test rationale
- Link test to roadmap step
- Tag flaky tests immediately

### 6.2 Validation Checklist
```markdown
### Pre-Push Checklist
- [ ] All tests pass locally
- [ ] No console.log() left behind
- [ ] Documentation updated
- [ ] Issue references added
- [ ] Roadmap position noted
```

## 📝 Rule 7: Knowledge Capture

### 7.1 "Aha!" Moment Documentation
```markdown
### Aha! Moment: [Brief Title]
**When:** [Timestamp]
**Context:** [What you were doing]
**Realization:** [What clicked]
**Application:** [How this helps]
**Keywords:** [For future searching]
```

### 7.2 Pattern Recognition
```markdown
### Pattern Spotted: [Pattern Name]
**Seen In:** [Where you've seen this]
**Solution Template:** [Reusable approach]
**Anti-pattern:** [What to avoid]
```

## 🚀 Rule 8: Continuous Improvement

### 8.1 Velocity Tracking
- Log estimated vs actual time
- Identify patterns in estimates
- Adjust future estimates based on data
- Celebrate when estimates improve

### 8.2 Ritual Refinement
- Every 10 sessions: Review these rules
- Remove what's not working
- Add what's missing
- Keep what's helping

## 🎮 Rule 9: The Vibe Check

### 9.1 Energy Management
```markdown
### Vibe Check
- Energy Level: [1-10]
- Focus Quality: [Laser | Good | Scattered]
- Creativity: [Flowing | Normal | Blocked]
- Action: [Push through | Take break | Switch tasks]
```

### 9.2 Context Switching
- Save "mental snapshot" before switching
- Note: Current thought, next action, open questions
- Use voice memo if faster than typing
- Maximum 2 minute transition ritual

## 🏁 Rule 10: The Golden Rules

1. **If it's not logged, it didn't happen**
2. **Future you is a different person - be kind to them**
3. **Confusion is data - document it**
4. **Small commits, clear messages**
5. **When stuck, zoom out to the roadmap**
6. **Celebrate small wins in the log**
7. **Time tracking is for learning, not judgment**
8. **The perfect system is the one you actually use**

## 📋 Quick Reference Templates

### Start of Day
```bash
# Copy-paste starter
echo "## Session: $(date '+%Y-%m-%d %H:%M')" >> session.log
echo "### Starting at: Roadmap Step X - [Name]" >> session.log
echo "### Goal: [Today's specific target]" >> session.log
```

### Issue Quick Log
```bash
# Quick issue logger
echo "### Issue: $1" >> issues.log
echo "Time: $(date '+%H:%M')" >> issues.log
echo "Attempting: $2" >> issues.log
```

### End of Day
```bash
# Session closer
echo "### Ending at: $(date '+%H:%M')" >> session.log
echo "### Stopped at: $1" >> session.log
echo "### Tomorrow start with: $2" >> session.log
```

---

*Remember: These rules are your tools, not your master. Adapt them to your flow, but maintain the discipline of documentation. Your future self will thank you.*
