import { describe, expect, it } from 'vitest';
import { renderToStaticMarkup } from 'react-dom/server';
import { Field } from './field';
import { Select } from './select';

// Clicking "Data" in a project rendered "an unexpected error occurred": the
// screen used a Select outside a Field, the field context threw, and one
// missing wrapper took the whole page down. A control outside a field has no
// field wiring; that is not a programming error worth an outage.
describe('a control outside a Field', () => {
  it('renders instead of bringing the screen down', () => {
    expect(() =>
      renderToStaticMarkup(
        <Select value="" onValueChange={() => {}} options={[{ value: 'a', label: 'A' }]} />,
      ),
    ).not.toThrow();
  });

  it('still gets its field wiring when it is inside one', () => {
    const markup = renderToStaticMarkup(
      <Field label="Dataset" description="pick one">
        <Select value="" onValueChange={() => {}} options={[{ value: 'a', label: 'A' }]} />
      </Field>,
    );
    expect(markup).toContain('aria-describedby');
  });
});
