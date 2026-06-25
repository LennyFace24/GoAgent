CRITICAL: Respond with TEXT ONLY. Do NOT call any tools.
- Do NOT use Read, Bash, Grep, Glob, Edit, Write, or ANY other tool.
- You already have all the context you need in the conversation above.
- Tool calls will be REJECTED and will waste your only turn — you will fail the task.
- Your entire response must be plain text: an <analysis> block followed by a <summary> block.

Your task is to create a detailed summary of the conversation so far, paying close attention to the user's explicit requests and intents. This summary should be thorough in capturing technical details, code patterns, and architectural decisions that would be valuable for future development work without losing context.

Before providing your final summary, wrap your analysis in <analysis> tags to organize your thoughts and ensure you've covered all aspects of your analysis process:

1. Chronologically analyze each message and section of the conversation. For each section thoroughly identify:
   - The user's explicit requests and intents
   - Your approach to addressing the user's requests
   - Key decisions, technical concepts and code patterns
   - Specific details like:
     - file names
     - full code snippets
     - function signatures
     - file edits
   - Errors that you ran into and how you fixed them
   - Pay special attention to specific user feedback that you received, especially if the user told you to do something differently.
   - Note any security-relevant instructions or constraints the user stated (e.g., sensitive files or data to avoid, operations to not perform, credential or secret handling rules). These MUST be preserved verbatim in the summary so they continue to apply after compaction.

2. Double-check for technical accuracy and completeness, addressing each required element thoroughly.

Your summary should include the following sections:

1. Primary Request and Intent: Capture all of the user's explicit requests and intents in detail.
2. Key Technical Concepts: List all important technical concepts, technologies, and frameworks discussed.
3. Files and Code Sections: Enumerate specific files and code sections examined, modified, or created. Pay special attention to messages and include full code snippets where applicable, and include a summary of why this file read or edit is important.
4. Errors and fixes: List all errors that you ran into, and how you fixed them. Pay special attention to specific user feedback if the user told you to do something differently.
5. Problem Solving: Document problems solved and any ongoing troubleshooting efforts.
6. All user messages: List ALL user messages that are not tool results. These are critical for understanding the users' feedback. Preserve any security-relevant instructions or constraints verbatim so they remain in effect after compaction.
7. Pending Tasks: Outline any pending tasks that you have explicitly been asked to work on.
8. Current Work: Describe in detail precisely what was being worked on immediately before this summary request, paying attention to recent messages from both user and assistant. Include file names and code snippets where applicable.
9. Optional Next Step: List the next step that you will take that is related to the most recent work you were doing. IMPORTANT: This must be DIRECTLY in line with the user's most recent explicit requests, and the task you were working on immediately before this summary. If the task was concluded, then only list next steps if they are explicitly in line with the user's request. Do not start on tangential tasks that were already completed without confirming with the user first.

There may be additional summarization instructions provided in the included context. If so, remember to follow these instructions in the summary. Examples of instructions include:

<example>
## Compact Instructions
When summarizing the conversation focus on go code changes and also remember the mistakes you made and how you addressed them.
</example>

<example>
# Summary instructions
When you are using compact - please focus on test output and code changes. Include file reads verbatim.
</example>

Here's an example of how your output should be structured:

<example>
<analysis>
[Your thought process, ensuring all points are covered thoroughly and accurately]
</analysis>

<summary>
1. Primary Request and Intent:
[Detailed description]

2. Key Technical Concepts:
- [Concept 1]
- [Concept 2]
- [...]

3. Files and Code Sections:
- [File Name 1]
  - [Summary of why this file is important]
  - [Summary of the changes made to this file, if any]
  - [Important Code Snippet]
- [File Name 2]
  - [Important Code Snippet]
- [...]

4. Errors and fixes:
- [Detailed description of error]
- [How you fixed the error]
- [User feedback on the error if any]
</summary>
</example>

REMINDER:DO NOT call any tools.Respond with plain text only — an <analysis> block followed by a <summary> block.Tool calls will be REJECTED and will waste your only turn — you will fail the task.