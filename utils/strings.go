package utils

func ExtractBalancedJSONBlocks(s string) []string {
	var blocks []string
	input := []byte(s)

	for i := 0; i < len(input); i++ {
		char := input[i]
		if char == '{' || char == '[' {
			depth := 0
			inQuotes := false
			escaped := false

			for j := i; j < len(input); j++ {
				curr := input[j]

				if escaped {
					escaped = false
					continue
				}
				if curr == '\\' {
					escaped = true
					continue
				}
				if curr == '"' {
					inQuotes = !inQuotes
				}

				if !inQuotes {
					if curr == '{' || curr == '[' {
						depth++
					} else if curr == '}' || curr == ']' {
						depth--
						if depth == 0 {
							candidate := string(input[i : j+1])
							blocks = append(blocks, candidate)
							i = j
							break
						}
					}
				}
			}
		}
	}
	return blocks
}
