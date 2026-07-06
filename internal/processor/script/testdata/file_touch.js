function main(input, api) {
  if (input.file) {
    input.file.content = input.file.content + "\n# script-touched";
  }
  return input;
}
