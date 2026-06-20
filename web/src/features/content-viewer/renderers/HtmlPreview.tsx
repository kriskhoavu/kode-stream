import { useMemo } from 'react';
import DOMPurify from 'dompurify';

const blockedResourceAttributes = ['src', 'srcset', 'poster', 'action', 'formaction'];

export function sanitizeHtmlDocument(content: string): string {
  const clean = DOMPurify.sanitize(content, {
    WHOLE_DOCUMENT: true,
    FORBID_TAGS: ['script', 'iframe', 'object', 'embed', 'form', 'base'],
    FORBID_ATTR: ['srcdoc']
  });
  const document = new DOMParser().parseFromString(clean, 'text/html');

  for (const element of document.querySelectorAll('*')) {
    for (const attribute of blockedResourceAttributes) element.removeAttribute(attribute);
    const href = element.getAttribute('href');
    if (href && !href.startsWith('#')) element.removeAttribute('href');
    const style = element.getAttribute('style');
    if (style && /url\s*\(|@import/i.test(style)) element.removeAttribute('style');
  }
  for (const style of document.querySelectorAll('style')) {
    if (/url\s*\(|@import/i.test(style.textContent ?? '')) style.remove();
  }

  const csp = document.createElement('meta');
  csp.httpEquiv = 'Content-Security-Policy';
  csp.content = "default-src 'none'; img-src data:; style-src 'unsafe-inline'; font-src data:;";
  document.head.prepend(csp);
  return `<!doctype html>${document.documentElement.outerHTML}`;
}

export function HtmlPreview({ content }: { content: string }) {
  const source = useMemo(() => sanitizeHtmlDocument(content), [content]);
  return <iframe className="content-viewer-html" title="HTML preview" sandbox="" srcDoc={source} />;
}
