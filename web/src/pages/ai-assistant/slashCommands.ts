/**
 * Slash-command registry type for the AI Assistant compose bar.
 *
 * Each command is a plain object — name (without the leading slash),
 * a short description shown in the palette, and an action callback.
 * Commands are defined in AiAssistantPage and passed to ComposeBar so
 * they have natural access to store actions and other page-level state.
 */
export interface SlashCommand {
  /** Command identifier — typed after the `/` (e.g. "clear"). */
  name: string;
  /** One-line description shown in the command palette. */
  description: string;
  /** Called when the user confirms the command. */
  onRun: () => void;
}
