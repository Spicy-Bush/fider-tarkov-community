export const DISMISSED_CONTENT_PREFIX = 'fider_dismissed';

export const isTouch = (): boolean => {
  return "ontouchstart" in window || navigator.maxTouchPoints > 0
}

export const generateHash = (str: string): string => {
  let hash = 0;
  if (str.length === 0) return hash.toString();
  
  for (let i = 0; i < str.length; i++) {
    const char = str.charCodeAt(i);
    hash = ((hash << 5) - hash) + char;
    hash = hash & hash; // 32 bit int should be fine?
  }
  
  return hash.toString();
}

export const isDismissed = (contentType: string, contentHash: string): boolean => {
  const dismissedHashes = getDismissedHashes(contentType);
  return dismissedHashes.includes(contentHash);
}

export const dismissContent = (contentType: string, contentHash: string): void => {
  const dismissedHashes = getDismissedHashes(contentType);
  if (!dismissedHashes.includes(contentHash)) {
    dismissedHashes.push(contentHash);
    localStorage.setItem(getStorageKey(contentType), JSON.stringify(dismissedHashes));
  }
}

export const getDismissedHashes = (contentType: string): string[] => {
  try {
    const stored = localStorage.getItem(getStorageKey(contentType));
    return stored ? JSON.parse(stored) : [];
  } catch (e) {
    return [];
  }
}

export const clearDismissedContent = (contentType: string): void => {
  localStorage.removeItem(getStorageKey(contentType));
}

const getStorageKey = (contentType: string): string => {
  return `${DISMISSED_CONTENT_PREFIX}_${contentType}`;
}

export const DismissableContentTypes = {
  MESSAGE_BANNER: 'message_banner'
} as const;

export const dismissContentByValue = (contentType: string, content: string): void => {
  const contentHash = generateHash(content);
  dismissContent(contentType, contentHash);
}

export const isContentDismissed = (contentType: string, content: string): boolean => {
  const contentHash = generateHash(content);
  return isDismissed(contentType, contentHash);
}