import { useState } from 'react';
import { writePreference } from '../../shared/preferences/store';
import { type BoardKind, type BoardLayout, boardKindPreference, layoutPreferences, readBoardKind, readBoardLayout } from './boardPreferences';

export function useBoardView(harnessOn: boolean) {
	const [boardKind, setBoardKindState] = useState<BoardKind>(() => readBoardKind());
	const [layouts, setLayouts] = useState<Record<BoardKind, BoardLayout>>(() => ({ tasks: readBoardLayout('tasks'), plans: readBoardLayout('plans') }));
	const shownKind: BoardKind = harnessOn ? boardKind : 'plans';
	const setBoardKind = (kind: BoardKind) => {
		setBoardKindState(kind);
		writePreference(boardKindPreference, kind);
	};
	const setLayout = (layout: BoardLayout) => {
		setLayouts((current) => ({ ...current, [shownKind]: layout }));
		writePreference(layoutPreferences[shownKind], layout);
	};
	return { setBoardKind, shownKind, shownLayout: layouts[shownKind], setLayout };
}
