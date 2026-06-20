import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { MarkdownPreview, renderMarkdown } from './MarkdownPreview';

describe('renderMarkdown', () => {
  it('renders GFM, KaTeX, and highlighted code', async () => {
    const html = await renderMarkdown(`
| Name | Value |
| --- | --- |
| one | 1 |

- [x] complete

$x^2$

\`\`\`typescript
const value = 1;
\`\`\`
`);

    expect(html).toContain('<table>');
    expect(html).toContain('type="checkbox"');
    expect(html).toContain('class="katex"');
    expect(html).toContain('class="hljs');
  });

  it('does not pass raw Markdown HTML into the rendered document', async () => {
    const html = await renderMarkdown('<script>alert(1)</script><img src=x onerror=alert(2)>');

    expect(html).not.toContain('<script');
    expect(html).not.toContain('onerror');
  });

  it('protects external links', async () => {
    const html = await renderMarkdown('[Open](https://example.com)');

    expect(html).toContain('target="_blank"');
    expect(html).toContain('rel="noreferrer noopener"');
  });

  it('copies the original fenced code', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, 'clipboard', { configurable: true, value: { writeText } });
    render(<MarkdownPreview content={'```typescript\nconst value = 1;\n```'} />);

    fireEvent.click(await screen.findByRole('button', { name: 'Copy code block' }));
    await waitFor(() => expect(writeText).toHaveBeenCalledWith('const value = 1;\n'));
  });
});
