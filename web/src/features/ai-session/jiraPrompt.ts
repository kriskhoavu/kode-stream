import type { JiraIssue } from '../../lib/types';

const jiraPromptBlockPattern = /(?:\n\n)?--- Jira ticket description ---\n[\s\S]*?\n--- End Jira ticket description ---/;

export function removeJiraDescriptionPrompt(prompt: string) {
	return prompt.replace(jiraPromptBlockPattern, '').trim();
}

export function appendJiraDescriptionPrompt(prompt: string, issue: JiraIssue) {
	const description = issue.description.trim();
	if (!description) return removeJiraDescriptionPrompt(prompt);
	const title = issue.summary.trim() ? `${issue.key}: ${issue.summary.trim()}` : issue.key;
	const block = `--- Jira ticket description ---\n${title}\n\n${description}\n--- End Jira ticket description ---`;
	const base = removeJiraDescriptionPrompt(prompt);
	return base ? `${base}\n\n${block}` : block;
}
