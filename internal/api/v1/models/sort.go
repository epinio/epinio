package models

// Implement the Sort interface for service response slices

func (srl ServiceResponseList) Len() int {
	return len(srl)
}

func (srl ServiceResponseList) Swap(i, j int) {
	srl[i], srl[j] = srl[j], srl[i]
}

func (srl ServiceResponseList) Less(i, j int) bool {
	return srl[i].Name < srl[j].Name
}
