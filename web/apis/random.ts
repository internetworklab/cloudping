export function generateRandomTaskId(): string {
  const min = 10000;
  const max = 65535;
  const randomInt = Math.floor(Math.random() * (max - min + 1)) + min;
  return randomInt.toString();
}
