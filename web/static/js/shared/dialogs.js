export function showDialog(id) {
  const dialog = document.getElementById(id);
  if (dialog.open) return;

  dialog.showModal();
  dialog.querySelector('.dialog-content')?.scrollTo(0, 0);
}

// Use the same cancellable dismissal path as Escape, including draft guards.
export function requestDialogClose(id) {
  const dialog = document.getElementById(id);
  if (!dialog?.open) return;

  const dismissal = new Event('cancel', { cancelable: true });
  if (dialog.dispatchEvent(dismissal)) dialog.close();
}

function isBackdropInteraction(event, dialog) {
  if (event.target !== dialog) return false;

  const bounds = dialog.getBoundingClientRect();
  return (
    event.clientX < bounds.left ||
    event.clientX >= bounds.right ||
    event.clientY < bounds.top ||
    event.clientY >= bounds.bottom
  );
}

export function bindDialogEvents() {
  for (const dialog of document.querySelectorAll('dialog')) {
    let pressedBackdrop = false;

    dialog.addEventListener('pointerdown', (event) => {
      pressedBackdrop = event.button === 0 && isBackdropInteraction(event, dialog);
    });

    dialog.addEventListener('click', (event) => {
      const dismiss = pressedBackdrop && isBackdropInteraction(event, dialog);
      pressedBackdrop = false;
      if (dismiss) requestDialogClose(dialog.id);
    });

    function resetBackdropPress() {
      pressedBackdrop = false;
    }

    dialog.addEventListener('pointercancel', resetBackdropPress);
    dialog.addEventListener('close', resetBackdropPress);
  }
}
