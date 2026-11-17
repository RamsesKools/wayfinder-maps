# Billing Rewrite: Phase 1!

Rework billing so invoices are final once issued.

## Destination

Invoices never change after issue.

## Tickets

- https://github.com/acme/widgets/11 is the first decision, see ./tickets/11-should-invoices-be-immutable.md and https://github.com/acme/widgets/issues/110.
- Research lives in ./tickets/12-survey-of-payment-providers.md
- ./tickets/13-prototype-a-credit-note-flow.md, ./tickets/14-write-the-migration-runbook.md, ./tickets/15-implement-immutable-invoice-store.md.

## Fog

Tax handling is unclear until #12 lands.

## Decisions so far

- [Should invoices be immutable?](./tickets/11-should-invoices-be-immutable.md)
- [Pick the PDF renderer](./tickets/17-pick-the-pdf-renderer.md)

## Implemented so far

- [Implement immutable invoice store](./tickets/15-implement-immutable-invoice-store.md)
- [Implement tax lines](./tickets/20-implement-tax-lines.md)

## Out of scope

- [Prototype a credit-note flow](./tickets/13-prototype-a-credit-note-flow.md)
- [Implement invoice emailing](./tickets/18-implement-invoice-emailing.md)
