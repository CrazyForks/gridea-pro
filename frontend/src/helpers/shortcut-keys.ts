export function getShortcutKeys(t: (key: string) => string) {
  return [
    {
      name: t('shortcutKeys.editing'),
      list: [
        {
          title: t('shortcutKeys.saveArticle'),
          keyboard: ['⌘', 'S'],
        },
        {
          title: t('shortcutKeys.cut'),
          keyboard: ['⌘', 'X'],
        },
        {
          title: t('shortcutKeys.copy'),
          keyboard: ['⌘', 'C'],
        },
        {
          title: t('shortcutKeys.paste'),
          keyboard: ['⌘', 'V'],
        },
      ],
    },
    {
      name: 'Markdown',
      list: [
        {
          title: t('shortcutKeys.headingDown'),
          keyboard: ['⌃', '⇧', '['],
        },
        {
          title: t('shortcutKeys.headingUp'),
          keyboard: ['⌃', '⇧', ']'],
        },
        {
          title: t('shortcutKeys.bold'),
          keyboard: ['⌘', 'B'],
        },
        {
          title: t('shortcutKeys.inlineCode'),
          keyboard: ['⌘', '`'],
        },
        {
          title: t('shortcutKeys.italic'),
          keyboard: ['⌘', 'I'],
        },
        {
          title: t('shortcutKeys.list'),
          keyboard: ['⌘', 'L'],
        },
        {
          title: t('shortcutKeys.latex'),
          keyboard: ['⌘', 'M'],
        },
        {
          title: t('shortcutKeys.latexBlock'),
          keyboard: ['⇧', '⌘', 'M'],
        },
        {
          title: t('shortcutKeys.strikethrough'),
          keyboard: ['⌥', 'S'],
        },
      ],
    },
    {
      name: t('shortcutKeys.other'),
      list: [
        {
          title: t('shortcutKeys.formatDocument'),
          keyboard: ['⇧', '⌥', 'F'],
        },
      ],
    },
  ]
}
