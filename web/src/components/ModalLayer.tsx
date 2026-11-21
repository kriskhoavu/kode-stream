import type { ReactNode } from 'react';
import { createPortal } from 'react-dom';

/**
 * Draws a dialog and its backdrop at the end of the document. A page's
 * content sits below the app's top bar in the shell's stacking order, so a
 * dialog drawn inside the page would slide under the bar.
 */
export function ModalLayer({ children }: { children: ReactNode }) {
	return createPortal(children, document.body);
}
