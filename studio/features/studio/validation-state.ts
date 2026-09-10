export function isCurrentValidation(
  requestedKey: string,
  currentKey: string,
  requestedID: string,
  returnedID: string,
): boolean {
  return requestedKey === currentKey && requestedID === returnedID;
}
