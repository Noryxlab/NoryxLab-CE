# Budgets

A project can be given a monthly allowance in vCPU-hours. Crossing it is
announced to the administrators and **nothing is ever stopped**.

Administration → Budgets sets them. A project's own figure is on its overview,
beside the consumption it bounds. The decision is `ADR-057`.

## The unit is the vCPU-hour, not the euro

Two cores for three hours is six vCPU-hours, which is the number that compares
across machines of different sizes. It comes from the sampler that has written
one measurement per project every five minutes since 2026-09-06: twelve samples
make an hour, so the sum of vCPU divided by twelve is the figure. Consumption is
never stored — a copied total would be a second number to keep true, and
the day the two diverged nobody would know which to believe.

Currency is deliberately absent. This platform runs on hardware somebody
already bought, so a price per hour would be a number the product invented. An
installation that wants euros sets a price per vCPU-hour and the same budget
reads in currency, with the conversion visible rather than built in.

## Unlimited is the default

A project with no allowance consumes with no ceiling. That is a chosen state,
not a field waiting to be filled: a budget nobody set must never refuse
anything. The screen says **Unlimited**, and the action that returns a project
to it is called *Back to unlimited* rather than *Remove the allowance* — the
same gesture, named by where it lands.

On the wire, `vcpuHours: 0` is the removal. A negative value is refused, so that
a typo cannot become an absurd limit that alerts without end.

## Two thresholds, and one alert each

75% and 100%. Two and not five: an alert that arrives often stops being read.
The first is early enough to change course; the second is the fact itself.

Each threshold is announced **once per project per calendar month**, to the
platform's alert channel. The alert carries the three figures that let somebody
check it without opening the platform — what was allowed, what was taken, over
which period — and it carries, in the alert itself,
`"action": "none: workloads keep running, this is a notice"`.

**Changing or removing an allowance forgets the thresholds already announced.**
"75% of 300" and "75% of 1000" are not the same event, so an allowance raised or
lowered mid-month would otherwise cross its new threshold in silence. Saving the
same value twice changes nothing and stays quiet.

## Nothing is enforced

Going over an allowance suspends no workspace, no job and no application. This
follows the rule the platform already applies to applications
([Applications](APPS_V1.md)): make the cost visible rather than cut it.
Killing a researcher's forty-minute mesh at 100% of a number an administrator
typed last month is the behaviour that makes people stop declaring budgets at
all.

A red bar therefore means "somebody should talk about this", never "something
was stopped". The screen says so above the table, and the project's own card
says it too — an administrator who believed a project had been suspended would
go looking for an outage that does not exist.

## Who may do what

| Act | Who |
| --- | --- |
| Set or clear an allowance | platform administrator |
| Read a project's own allowance and consumption | any member of that project |
| Read every project's | platform administrator |

Posing a budget arbitrates between projects over a shared resource, which a
member of one project cannot do for the others. Reading one's own needs
membership and nothing more: a team that cannot see its consumption cannot
change it.

## What this does not cover

Storage, which is measured separately and has no allowance. Why a project
consumed, which the activity report answers. And the project itself is not
notified — only the administrators are. A project owner whose team is at 90%
learns it by opening the overview.
