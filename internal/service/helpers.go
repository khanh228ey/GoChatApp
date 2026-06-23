package service

import (
	"fmt"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// parseObjectID chuyển string hex thành primitive.ObjectID.
func parseObjectID(id string) (primitive.ObjectID, error) {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return primitive.NilObjectID, fmt.Errorf("invalid object id: %w", err)
	}
	return oid, nil
}
