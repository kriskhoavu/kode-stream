import type { ReactNode } from 'react';
import { createPortal } from 'react-dom';

// Portal to body: inside a page, the dialog would slide under the top bar.
export function ModalLayer({ children }: { children: ReactNode }) {
	return createPortal(children, document.body);
}
