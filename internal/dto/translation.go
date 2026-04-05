package dto

// TopicTranslation represents a single topic translation entry.
type TopicTranslation struct {
	TopicID        int64  `json:"topicId"`
	LanguageCode   string `json:"languageCode"`
	TranslatedName string `json:"translatedName"`
}

// UpdateTopicTranslationsRequest is the request body for updating topic translations.
type UpdateTopicTranslationsRequest struct {
	Translations []TopicTranslation `json:"translations"`
}

// MraUpdateTopicTranslationRequest is the request body for updating topic translations (MRA).
type MraUpdateTopicTranslationRequest struct {
	TopicInfo [][]any `json:"topicInfo"`
	UserID    any     `json:"userId"`
}
