

export const statusLabels = {
  unsorted: 'Unsorted',
  draft: 'Draft',
  in_progress: 'In Progress',
  review: 'Review',
  done: 'Done'
} as const;

export const statusOrder = Object.keys(statusLabels) as Array<keyof typeof statusLabels>;
export const editableStatusOrder = statusOrder.filter((status) => status !== 'unsorted');
