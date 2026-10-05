# References

External documents the platform's design relies on. Kept beside the code so
that a decision can be traced to its source.

## Agents and workflows

- Anthropic, *Building Effective AI Agents: Architecture Patterns and
  Implementation Frameworks*, 2025.
  https://resources.anthropic.com/hubfs/Building%20Effective%20AI%20Agents-%20Architecture%20Patterns%20and%20Implementation%20Frameworks.pdf

  The reference for the agent and workflow layer, adopted in Noryx ADR-046
  (*Workflows over agents in Noryx*). In particular its decision framework —
  *"High control requirements (regulatory compliance, financial transactions,
  safety-critical operations) → start with single agents or sequential
  workflows"* — which is the cell Noryx's customers occupy, and its rule that
  an architecture should *"start simple, measure everything, add complexity
  only when it delivers measurable value."*
