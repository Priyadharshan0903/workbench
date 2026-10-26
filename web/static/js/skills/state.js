export function createSkillState() {
  return {
    active: false,
    metadata: null,
    document: null,
    instructions: null,
    mode: 'preview',
    draft: '',
    dirty: false,
    loading: false,
    busy: false,
    sequence: 0,
    error: '',
    message: '',
    discardAction: null,
    previewHTML: '',
    destination: '',
    copyName: '',
    context: null,
  };
}
