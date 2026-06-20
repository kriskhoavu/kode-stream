import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { HtmlPreview, sanitizeHtmlDocument } from './HtmlPreview';

describe('HtmlPreview', () => {
  it('removes active content and remote resources', () => {
    const html = sanitizeHtmlDocument(`
      <script>alert(1)</script>
      <img src="https://example.com/image.png" onerror="alert(2)">
      <a href="https://example.com">remote</a>
      <div style="background:url(https://example.com/a.png)">content</div>
    `);

    expect(html).not.toContain('<script');
    expect(html).not.toContain('onerror');
    expect(html).not.toContain('https://example.com');
    expect(html).toContain('Content-Security-Policy');
  });

  it('uses an iframe sandbox without permissions', () => {
    render(<HtmlPreview content="<h1>Preview</h1>" />);

    const frame = screen.getByTitle('HTML preview');
    expect(frame).toHaveAttribute('sandbox', '');
    expect(frame).toHaveAttribute('srcdoc', expect.stringContaining('<h1>Preview</h1>'));
  });
});
