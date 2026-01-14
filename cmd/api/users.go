package main

import (
	"errors"
	"greenlight/internal/data"
	"greenlight/internal/validator"
	"net/http"
)

func (app application) registerUserHandler(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	user := &data.User{
		Name:      input.Name,
		Email:     input.Email,
		Activated: false,
	}

	err = user.Password.Set(input.Password)
	if err != nil {
		app.internalServerErrorResponse(w, r, err)
		return
	}

	v := validator.New()

	if data.ValidateUser(v, user); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	err = app.model.Users.Insert(user)
	if err != nil {
		switch {
		case errors.Is(data.ErrDuplicateEmail, err):
			// TODO: prevent enumeration attacks
			// returning such a message confirms that a user with the given email exists
			// often leading to an attacker trying to compromise the user's account through social engineering
			v.AddError("email", "a user with this email already exists")
			app.failedValidationResponse(w, r, v.Errors)
		default:
			app.internalServerErrorResponse(w, r, err)
		}
		return
	}

	app.background(func() {
		if err = app.mailer.Send(user.Email, "user_welcome.tmpl.html", user); err != nil {
			// don't send http responses inside background processes
			// log the error instead
			app.logger.Error(err.Error())
		}
	})

	err = app.writeJSON(w, http.StatusAccepted, envelope{"user": user}, nil)
	if err != nil {
		app.internalServerErrorResponse(w, r, err)
	}
}
